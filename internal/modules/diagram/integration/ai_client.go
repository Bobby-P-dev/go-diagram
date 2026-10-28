package integration

import (
	"encoding/json"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
	diagramservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/llm"
	"strings"
)

type CompletionClient interface {
	CallLLM(systemPrompt string, history []llm.Message, prompt string) (string, error)
	SanitizeJSON(raw string) string
	Provider() string
	Model() string
}

type AIClient struct{ client CompletionClient }

var _ diagramservice.AIClient = (*AIClient)(nil)

func NewAIClient(client CompletionClient) *AIClient {
	return &AIClient{client: client}
}

func (adapter *AIClient) Provider() string { return adapter.client.Provider() }
func (adapter *AIClient) Model() string    { return adapter.client.Model() }

func (adapter *AIClient) GenerateDiagram(
	historyMessages []diagram.ChatMessage,
	currentGraph string,
	newPrompt string,
	diagramType string,
	targetedNodeIDs []string,
) (*dto.GraphPayload, string, error) {
	systemPrompt := baseSystemPrompt
	if diagramType != "" {
		systemPrompt += fmt.Sprintf("\n\nTARGET DIAGRAM TYPE: %s", strings.ToUpper(diagramType))
		switch strings.ToLower(diagramType) {
		case "erd", "database":
			systemPrompt += "\n- MANDATORY: Design relational database tables. Every entity node MUST use type: 'database' with rich 'columns' (PK, FK, constraints, types) and relational edges (1:N, 1:1, N:M)." +
				"\n- STRICT ERD RULE: DO NOT generate any 'lane' property. ERD diagrams MUST be pure relational table structures without swimlanes, lanes, or departmental bands. Keep 'lane' empty or omit it completely."
		case "flowchart", "workflow":
			systemPrompt += "\n- MANDATORY: Design a procedural step-by-step flowchart. Use 'input' (start), 'decision' (branches with Yes/No edges), 'default' (action steps), and 'output' (end/terminal)." +
				"\n- FLOWCHART CONNECTOR ROUTING RULES:" +
				"\n  1. For 'decision' branches: Primary/positive branch (e.g. 'Yes', 'Valid', 'Success') MUST use sourceHandle: 'bottom' and targetHandle: 'top'. Secondary/negative/retry branch (e.g. 'No', 'Invalid', 'Failed') MUST use sourceHandle: 'right' (or 'left') and targetHandle: 'top' to prevent overlapping connector lines." +
				"\n  2. For regular sequential steps: connect sourceHandle: 'bottom' to targetHandle: 'top'." +
				"\n  3. Every step must logically connect to a downstream node or terminal output node. Do NOT leave dead ends."
		case "architecture", "system":
			systemPrompt += "\n- MANDATORY: Design system architecture. Use 'input' (clients/CDN), 'decision' (gateways/load balancers), 'default' (services/message queues), and 'database' (storage/cache)."
		case "sequence", "interaction":
			systemPrompt += "\n- MANDATORY: Design a Sequence Diagram depicting chronological message interactions.\n" +
				"  1. Define lifeline nodes: 'input' for User/Actor, 'default' for API Gateway, Services, Webhooks, and 'database' for Data Stores.\n" +
				"  2. Model chronological requests and responses with numbered sequential labels (e.g. '1. POST /login', '2. Validate Credentials', '3. 200 OK + JWT').\n" +
				"  3. Use solid edges ('dashed': false) for sync requests, and dashed edges ('dashed': true) for returns/async responses.\n" +
				"  4. Arrange sequence progression from left to right."
		case "c4", "context", "container":
			systemPrompt += "\n- MANDATORY: Design a C4 Architecture Model (Context & Container Diagram).\n" +
				"  1. 'input': External Actors or Third-Party Systems.\n" +
				"  2. 'decision': Reverse Proxy, API Gateway, or Edge Ingress.\n" +
				"  3. 'default': Container Applications (e.g. 'Single-Page App [Vue/React]', 'Backend API [Go]', 'Worker [Python]').\n" +
				"  4. 'database': Databases, Cache Stores, and Message Buses (e.g. 'PostgreSQL [Relational Database]', 'Redis [Cache]').\n" +
				"  5. Edges: Explicit protocols and roles (e.g. 'Delivers SPA via HTTPS', 'Sends REST/JSON calls', 'Reads/Writes data via TCP 5432')."
		case "network", "infra", "infrastructure", "security":
			systemPrompt += "\n- MANDATORY: Design Cloud Network & Infrastructure Topology.\n" +
				"  1. 'input': Public Internet, Route53 DNS, Cloudflare CDN.\n" +
				"  2. 'decision': WAF, Edge Firewalls, NAT Gateways, Application Load Balancers.\n" +
				"  3. 'default': Compute instances, Bastion hosts, Kubernetes pods, and worker nodes inside Private Subnets.\n" +
				"  4. 'database': Managed Database Clusters (RDS Aurora, Multi-AZ PostgreSQL, Redis Cluster).\n" +
				"  5. Edges: Security groups, CIDRs, and port mappings (e.g. 'Port 443 HTTPS', 'Port 6443 Kube-API', 'VPC Peering')."
		case "cicd", "devops", "gitflow":
			systemPrompt += "\n- MANDATORY: Design a CI/CD DevOps & Deployment Pipeline.\n" +
				"  1. 'input': Git Repository Trigger (e.g. 'PR Merged to main', 'Git Tag v1.0.0').\n" +
				"  2. 'default': Automated pipeline stages (e.g. 'Lint & Static Analysis', 'Unit & Integration Tests', 'Build Docker Image', 'Security Scan').\n" +
				"  3. 'decision': Quality gates & approvals (e.g. 'Tests Passed?', 'Approval Gate').\n" +
				"  4. 'output': Deployment targets (e.g. 'Deploy Staging K8s', 'Canary Rollout 10%', 'Slack Notification').\n" +
				"  5. Edges: Ordered pipeline progression with condition labels."
		case "state", "lifecycle":
			systemPrompt += "\n- MANDATORY: Design state machine/lifecycle transitions. Nodes represent states (Draft, Active, Finished) and edges represent transition events/triggers."
		case "uml", "class":
			systemPrompt += "\n- MANDATORY: Design UML class diagram. Use type: 'database' where columns list attributes and methods."
		case "pipeline", "dfd":
			systemPrompt += "\n- MANDATORY: Design data pipeline / ETL data flow. Use 'input' (data sources), 'default' (transformations/stream processing), 'database' (lake/warehouse), and 'output' (analytics/dashboards)."
		case "mindmap", "concept":
			systemPrompt += "\n- MANDATORY: Design concept mindmap. Use 'input' (root concept), 'decision' (major branch points), 'default' (subtopics), and 'output' (leaves/action items)."
		case "swimlane", "bpmn":
			systemPrompt += "\n- MANDATORY: Design a Cross-Functional Swimlane BPMN Flowchart ala Eraser.io.\n" +
				"  1. Identify 2 to 5 primary actors/departments (e.g. 'REQUESTER', 'APPROVER', 'PURCHASING', 'SUPPLIER', 'ERP SYSTEM').\n" +
				"  2. EVERY node MUST have 'data.lane' populated with its exact uppercase actor/department name.\n" +
				"  3. Use 'input' for Start event, 'default' for Action Tasks, 'decision' for Gateway diamonds (e.g. 'PR Approved?'), and 'output' for End/Closed states.\n" +
				"  4. Add 'data.icon' for each node (e.g. 'pencil', 'send', 'search', 'award', 'clipboard', 'cart', 'lock', 'clock', 'check', 'x').\n" +
				"  5. Edges crossing between different lanes MUST have 'dashed': true. Edges within the same lane MUST have 'dashed': false.\n" +
				"  6. Decision edges MUST have explicit condition labels like 'Yes - PR approved', 'No - returned'.\n" +
				"  7. Order the process logically from left to right across steps."
		case "usecase", "use_case":
			systemPrompt += "\n- MANDATORY: Design a standard UML Use Case Diagram.\n" +
				"  1. ACTORS (type: 'actor'): Identify human personas or external systems (e.g. 'Admin Gudang', 'Petugas Gudang', 'Supplier'). Nodes MUST use type: 'actor'. Use clean role title in 'label' and role description in 'subText'.\n" +
				"  2. USE CASES (type: 'usecase'): Elliptical action goals representing system functionality (e.g. 'Kelola Master Produk', 'Catat Mutasi Masuk', 'Catat Mutasi Keluar', 'Audit Stock Opname', 'Lihat Laporan Stok'). Nodes MUST use type: 'usecase'. Use concise verb-noun phrases.\n" +
				"  3. SYSTEM BOUNDARY: All 'usecase' nodes represent functions inside the system boundary.\n" +
				"  4. RELATIONSHIPS & EDGES:\n" +
				"     - Association (Actor to Use Case): Direct solid edge ('dashed': false), connecting the actor directly to the use cases they trigger.\n" +
				"     - <<include>>: Directed dashed edge ('dashed': true, label: '<<include>>') from parent use case to mandatory sub-use case (e.g. 'Catat Mutasi' -> 'Validasi Ketersediaan Stok').\n" +
				"     - <<extend>>: Directed dashed edge ('dashed': true, label: '<<extend>>') for conditional/optional extension flows (e.g. 'Alert Stok Kritis' -> 'Catat Mutasi Keluar').\n" +
				"  5. CONNECTOR ROUTING: Route Actor connections from sourceHandle: 'right' (or 'left') to targetHandle: 'left' (or 'right') of the use case oval. Inter-usecase include/extend edges connect between use cases.\n" +
				"  6. STRICT RULE: DO NOT use 'database' table nodes. UML Use Case diagrams only use 'actor' and 'usecase' nodes."
		}
	}

	if currentGraph != "" && currentGraph != "{}" && currentGraph != "[]" && currentGraph != `{"nodes":[],"edges":[]}` {
		systemPrompt += fmt.Sprintf("\n\nSTATE DIAGRAM SAAT INI:\n%s", currentGraph)
	} else {
		systemPrompt += "\n\nSTATE DIAGRAM SAAT INI: Belum ada diagram. Buat diagram baru dari awal."
	}

	if len(targetedNodeIDs) > 0 {
		systemPrompt += fmt.Sprintf("\n\nUSER FOCUS: The user has selected nodes with IDs: %s. Their next request is SPECIFICALLY targeting these nodes and their surrounding connections. Apply the user's modifications focused on this area while keeping the rest of the graph intact.", strings.Join(targetedNodeIDs, ", "))
	}

	messages := make([]llm.Message, len(historyMessages))
	for index, message := range historyMessages {
		messages[index] = llm.Message{Role: message.Role, Content: message.Content}
	}
	rawText, err := adapter.client.CallLLM(systemPrompt, messages, newPrompt)

	if err != nil {
		return nil, "", err
	}

	cleanedJSON := adapter.client.SanitizeJSON(rawText)

	type aiGraphWrapper struct {
		Message string          `json:"message"`
		Nodes   []dto.GraphNode `json:"nodes"`
		Edges   []dto.GraphEdge `json:"edges"`
	}

	var graph aiGraphWrapper
	if err := json.Unmarshal([]byte(cleanedJSON), &graph); err != nil {
		return nil, "", fmt.Errorf("failed to parse graph JSON from AI response: %w\nraw response: %s", err, rawText)
	}

	sanitizedNodes, sanitizedEdges := validateAndSanitizeGraph(graph.Nodes, graph.Edges, diagramType)
	if len(sanitizedNodes) == 0 {
		return nil, "", fmt.Errorf("AI returned no valid nodes")
	}

	chatAssistantMessage := strings.TrimSpace(graph.Message)
	if chatAssistantMessage == "" {
		chatAssistantMessage = fmt.Sprintf("Diagram updated with %d nodes and %d connections.", len(sanitizedNodes), len(sanitizedEdges))
	}

	return &dto.GraphPayload{
		Nodes: sanitizedNodes,
		Edges: sanitizedEdges,
	}, chatAssistantMessage, nil
}

func validateAndSanitizeGraph(nodes []dto.GraphNode, edges []dto.GraphEdge, diagramType string) ([]dto.GraphNode, []dto.GraphEdge) {
	if len(nodes) == 0 {
		return nodes, nil
	}

	isSwimlane := strings.ToLower(diagramType) == "swimlane" || strings.ToLower(diagramType) == "bpmn"

	// 1. Deduplicate node IDs and build valid node ID set
	seenNodeIDs := make(map[string]bool)
	sanitizedNodes := make([]dto.GraphNode, 0, len(nodes))

	for _, node := range nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			continue
		}
		if seenNodeIDs[id] {
			continue // deduplicate node ID
		}
		seenNodeIDs[id] = true

		// Strip lane for non-swimlane diagrams
		if !isSwimlane {
			node.Data.Lane = ""
		}
		sanitizedNodes = append(sanitizedNodes, node)
	}

	// 2. Filter orphan edges and ensure unique edge IDs
	seenEdgeIDs := make(map[string]bool)
	sanitizedEdges := make([]dto.GraphEdge, 0, len(edges))

	for idx, edge := range edges {
		src := strings.TrimSpace(edge.Source)
		tgt := strings.TrimSpace(edge.Target)

		// Both source and target must exist in sanitizedNodes (Zero Orphan Edges)
		if !seenNodeIDs[src] || !seenNodeIDs[tgt] {
			continue // Prune orphan edge
		}

		edgeID := strings.TrimSpace(edge.ID)
		if edgeID == "" || seenEdgeIDs[edgeID] {
			edgeID = fmt.Sprintf("e-%s-%s-%d", src, tgt, idx)
		}
		seenEdgeIDs[edgeID] = true
		edge.ID = edgeID

		lblLower := strings.ToLower(edge.Label)
		if strings.Contains(lblLower, "include") || strings.Contains(lblLower, "extend") {
			edge.Dashed = true
		}

		sanitizedEdges = append(sanitizedEdges, edge)
	}

	return sanitizedNodes, sanitizedEdges
}
