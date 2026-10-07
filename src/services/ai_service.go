package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/config"
	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type AIService struct {
	client    *http.Client
	provider  string
	apiKey    string
	model     string
	baseURL   string
	maxTokens int
}

func NewAIService() *AIService {
	provider := strings.ToLower(strings.TrimSpace(config.Env.AIProvider))
	if provider == "" {
		if config.Env.OpenAIAPIKey != "" {
			provider = "openai"
		} else {
			provider = "anthropic"
		}
	}

	var apiKey, model, baseURL string
	if provider == "openai" {
		apiKey = config.Env.OpenAIAPIKey
		model = config.Env.OpenAIModel
		baseURL = config.Env.OpenAIMBaseURL
		if !strings.HasSuffix(baseURL, "/chat/completions") {
			baseURL = strings.TrimSuffix(baseURL, "/") + "/chat/completions"
		}
	} else {
		provider = "anthropic"
		apiKey = config.Env.AnthropicAPIKey
		model = config.Env.AnthropicModel
		baseURL = config.Env.AnthropicBaseURL
	}

	maxTokens := config.Env.AIMaxTokens
	if maxTokens <= 0 {
		maxTokens = 16384
	}

	return &AIService{
		client: &http.Client{
			Timeout: 300 * time.Second,
		},
		provider:  provider,
		apiKey:    apiKey,
		model:     model,
		baseURL:   baseURL,
		maxTokens: maxTokens,
	}
}

type anthropicRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    string              `json:"system"`
	Messages  []map[string]string `json:"messages"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Error   *anthropicError    `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

const baseSystemPrompt = `Kamu adalah AI Diagram Architect & Editor profesional.
Tugasmu adalah membuat, memperluas, dan memodifikasi struktur diagram interaktif (nodes & edges) berdasarkan instruksi user dan STATE DIAGRAM SAAT INI.
Output WAJIB JSON murni (nodes & edges) tanpa pembungkus markdown apapun.

STRICT RULES:
1. Do NOT wrap your response in markdown code blocks (no ` + "```" + ` or ` + "```json" + `).
2. Do NOT include any explanation, commentary, or text outside the JSON.
3. Your entire response must be a single valid JSON object.
4. Maintain a clean, logical, and properly linked layout for Vue Flow.
5. NO ORPHAN EDGES: Every edge's "source" and "target" MUST exist in the "nodes" array.

JSON OUTPUT STRUCTURE:
{
  "message": "Brief description of the action taken",
  "nodes": [
    {
      "id": "string (unique semantic ID, e.g. 'users', 'auth_service', 'step_login')",
      "type": "string (one of: 'default', 'input', 'output', 'decision', 'database')",
      "data": {
        "label": "string (main title / entity name)",
        "subText": "string (optional description, technology, role, or summary)",
        "lane": "string (STRICT: ONLY for swimlane/bpmn diagrams. NEVER use for ERD, database, flowchart, or architecture. Leave empty "" or omit for non-swimlane)",
        "icon": "string (optional action icon: 'pencil', 'send', 'search', 'award', 'clipboard', 'cart', 'lock', 'clock', 'check', 'x')",
        "columns": [
          {
            "name": "string (field name / method name)",
            "type": "string (data type / return type)",
            "is_pk": true,
            "is_fk": false,
            "constraint": "string (e.g. 'PK', 'FK', 'UNIQUE', 'NOT NULL', 'Indexed')"
          }
        ]
      }
    }
  ],
  "edges": [
    {
      "id": "string (unique identifier, e.g. 'e-users-orders')",
      "source": "string (id of source node)",
      "target": "string (id of target node)",
      "label": "string (optional descriptive relationship, protocol, or condition)"
    }
  ]
}

UNIVERSAL DIAGRAM GUIDELINES:

1. ERD / DATABASE SCHEMA:
   - Node Type: ALWAYS use "database".
   - "columns": Must be populated with fields, datatypes (UUID, VARCHAR, INT, TIMESTAMP, etc.), is_pk, is_fk, and constraints.
   - Edges: Connect foreign keys with cardinalities like "1:1", "1:N", "N:M", "references".

2. SYSTEM ARCHITECTURE / CLOUD & MICROSERVICES:
   - "input": Client apps (Web SPA, Mobile App, External Webhook, IoT, DNS/CDN).
   - "decision": Gateways & Traffic routers (API Gateway, Nginx, Load Balancer, Ingress).
   - "default": Backend Services, Microservices, Message Brokers (Kafka, RabbitMQ), Background Workers.
   - "database": Storage & Caching (PostgreSQL, Redis, MongoDB, Elasticsearch, S3).
   - "output": Third-party APIs (Stripe, SendGrid), Notification Sinks, Monitoring/Analytics.
   - Edges: Communication protocol/action (e.g., "HTTPS/REST", "gRPC", "Pub/Sub", "TCP", "SQL Query", "Cache Read").

3. FLOWCHARTS & BUSINESS PROCESSES (BPMN):
   - "input": Start / Trigger event.
   - "default": Action step, processing, task execution.
   - "decision": Branching condition, decision diamond (Edge labels: "Ya/Tidak", "Approved/Rejected", "Valid/Invalid").
   - "output": End / Success / Failed terminal state.
   - Edges: Sequence direction with flow condition labels.

4. STATE MACHINE / LIFECYCLE DIAGRAMS:
   - Nodes: Represent lifecycle states (e.g., 'Draft', 'Pending Payment', 'Processing', 'Delivered', 'Cancelled').
   - "input": Initial state.
   - "default": Intermediate active states.
   - "output": Final / Terminal states.
   - Edges: Triggering events/actions (e.g., "submit()", "payment_success", "cancel_timeout").

5. UML CLASS DIAGRAM:
   - Node Type: Use "database" or "default". If "database", use "columns" for properties and methods (e.g., name: "+ calculateTotal()", type: "float").
   - Edges: Relationships like "inherits", "implements", "aggregates", "1..*".

6. DATA PIPELINE / ETL & DATA FLOW DIAGRAMS (DFD):
   - "input": Data sources (CDC, IoT stream, CSV ingest).
   - "default": Transformation / processing stages (Spark job, Kafka stream worker, dbt).
   - "database": Data Lake / Warehouse (Snowflake, BigQuery, Postgres).
   - "output": BI Dashboards, ML Model inference, Reporting services.
   - Edges: Data stream names or batch frequency (e.g., "Raw Stream", "Parquet Batch", "Realtime Webhook").

7. MINDMAP / CONCEPT HIERARCHY:
   - "input": Root concept or main topic.
   - "decision": Core branching or major categorical decision points.
   - "default": Subtopics, branches, or team functional groups.
   - "output": Leaves, deliverables, or action items.

8. SEQUENCE DIAGRAM / DISTRIBUTED TRACING:
   - Lifeline nodes: "input" for User/Client, "default" for API Gateway, Backend Services, External Webhooks, and "database" for DB/Storage.
   - Edges: Chronological request/response messages with numbered sequence prefixes (e.g. "1. POST /login", "2. Verify Hash", "3. 200 OK + Session Cookie").
   - Synchronous requests: solid lines. Asynchronous/Return messages: dashed lines.

9. C4 ARCHITECTURE MODEL (CONTEXT & CONTAINERS):
   - "input": External Actors, Customer Personas, or Third-Party Banking/Payment systems.
   - "decision": Reverse Proxy, Edge Gateways, or API Load Balancers.
   - "default": Main Software Containers (e.g., 'Web SPA [Vue 3]', 'Backend API [Go]', 'Worker [Python]').
   - "database": Data Stores, Message Brokers, and Caches (e.g., 'PostgreSQL [Relational Database]', 'Redis [Cache]').
   - Edges: Explicit communication protocols (e.g., 'HTTPS/REST', 'gRPC', 'AMQP / RabbitMQ', 'SQL TCP:5432').

10. NETWORK & CLOUD INFRASTRUCTURE TOPOLOGY:
   - "input": Public Internet, Anycast DNS, CDN (Cloudflare).
   - "decision": WAF, NAT Gateways, Ingress Controllers, Application Load Balancers.
   - "default": VPC Subnets, Bastion Hosts, Kubernetes Pods, Worker Nodes.
   - "database": Managed Database Clusters (RDS Aurora Multi-AZ, Elasticache, S3 Buckets).
   - Edges: Network security groups, CIDRs, and port mappings (e.g., 'HTTPS 443', 'Kube-API 6443', 'VPC Peering').

11. CI/CD & DEVOPS PIPELINES (CICD):
   - "input": Git Event / Trigger (e.g., 'Git Push to main', 'Pull Request Created').
   - "default": Automated pipeline stages (e.g., 'Lint & Static Check', 'Run Unit Tests', 'Build Docker Image', 'Security Scan').
   - "decision": Quality gates or manual approvals (e.g., 'Tests Passed?', 'Approval Gate').
   - "output": Deployment destinations (e.g., 'Deploy Staging K8s', 'Canary Rollout 10%', 'Slack Alert Notification').
   - Edges: Stage execution dependencies and condition labels.

SMART INCREMENTAL EDITING RULES:
- If STATE DIAGRAM SAAT INI already exists:
  * When user asks to ADD: Keep all existing nodes and edges, append new nodes and wire new edges.
  * When user asks to UPDATE/MODIFY: Update the specific node's data (label, subText, columns) without changing its ID or removing other nodes.
  * When user asks to DELETE: Remove the requested node and delete all edges connected to that node.
  * DO NOT wipe out or regenerate the whole diagram from scratch unless explicitly requested (e.g., "buat ulang dari awal" or "reset diagram").`

func (s *AIService) GenerateDiagram(
	historyMessages []entities.ChatMessage,
	currentGraph string,
	newPrompt string,
	diagramType string,
	targetedNodeIDs []string,
) (*dtos.GraphPayload, string, error) {
	systemPrompt := baseSystemPrompt
	if diagramType != "" {
		systemPrompt += fmt.Sprintf("\n\nTARGET DIAGRAM TYPE: %s", strings.ToUpper(diagramType))
		switch strings.ToLower(diagramType) {
		case "erd", "database":
			systemPrompt += "\n- MANDATORY: Design relational database tables. Every entity node MUST use type: 'database' with rich 'columns' (PK, FK, constraints, types) and relational edges (1:N, 1:1, N:M)." +
				"\n- STRICT ERD RULE: DO NOT generate any 'lane' property. ERD diagrams MUST be pure relational table structures without swimlanes, lanes, or departmental bands. Keep 'lane' empty or omit it completely."
		case "flowchart", "workflow":
			systemPrompt += "\n- MANDATORY: Design a procedural step-by-step flowchart. Use 'input' (start), 'decision' (branches with Yes/No edges), 'default' (action steps), and 'output' (end/terminal)."
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

	var rawText string
	var err error

	if s.provider == "openai" {
		rawText, err = s.callOpenAI(systemPrompt, historyMessages, newPrompt, &dtos.ResponseFormatOpenAI{Type: "json_object"})
	} else {
		rawText, err = s.callAnthropic(systemPrompt, historyMessages, newPrompt)
	}

	if err != nil {
		return nil, "", err
	}

	cleanedJSON := sanitizeJSONResponse(rawText)

	type aiGraphWrapper struct {
		Message string           `json:"message"`
		Nodes   []dtos.GraphNode `json:"nodes"`
		Edges   []dtos.GraphEdge `json:"edges"`
	}

	var graph aiGraphWrapper
	if err := json.Unmarshal([]byte(cleanedJSON), &graph); err != nil {
		return nil, "", fmt.Errorf("failed to parse graph JSON from AI response: %w\nraw response: %s", err, rawText)
	}

	if len(graph.Nodes) == 0 {
		return nil, "", fmt.Errorf("AI returned empty nodes array")
	}

	// For non-swimlane diagrams (especially ERD/database), ensure 'lane' is stripped to prevent accidental swimlanes
	isSwimlane := strings.ToLower(diagramType) == "swimlane" || strings.ToLower(diagramType) == "bpmn"
	if !isSwimlane {
		for i := range graph.Nodes {
			graph.Nodes[i].Data.Lane = ""
		}
	}

	chatAssistantMessage := strings.TrimSpace(graph.Message)
	if chatAssistantMessage == "" {
		chatAssistantMessage = fmt.Sprintf("Diagram updated with %d nodes and %d connections.", len(graph.Nodes), len(graph.Edges))
	}

	return &dtos.GraphPayload{
		Nodes: graph.Nodes,
		Edges: graph.Edges,
	}, chatAssistantMessage, nil
}

type LLMStreamCallback func(delta string, totalTokens int)

func (s *AIService) callOpenAI(
	systemPrompt string,
	historyMessages []entities.ChatMessage,
	newPrompt string,
	responseFormat *dtos.ResponseFormatOpenAI,
) (string, error) {
	return s.callOpenAIWithCallback(systemPrompt, historyMessages, newPrompt, responseFormat, nil)
}

func (s *AIService) callOpenAIWithCallback(
	systemPrompt string,
	historyMessages []entities.ChatMessage,
	newPrompt string,
	responseFormat *dtos.ResponseFormatOpenAI,
	onChunk LLMStreamCallback,
) (string, error) {
	// Di OpenAI, System Prompt dimasukkan sebagai message pertama dengan role "system"
	var openAIMessages []dtos.MessageOpenAI
	openAIMessages = append(openAIMessages, dtos.MessageOpenAI{
		Role:    "system",
		Content: systemPrompt,
	})

	for _, msg := range historyMessages {
		role := msg.Role
		if role != "user" && role != "assistant" && role != "system" {
			role = "user"
		}
		openAIMessages = append(openAIMessages, dtos.MessageOpenAI{
			Role:    role,
			Content: msg.Content,
		})
	}

	if strings.TrimSpace(newPrompt) != "" {
		openAIMessages = append(openAIMessages, dtos.MessageOpenAI{
			Role:    "user",
			Content: strings.TrimSpace(newPrompt),
		})
	}

	type chatCompletionChunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
				Role    string `json:"role"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	reqBody := dtos.ChatRequestOpenAI{
		Model:          s.model,
		Messages:       openAIMessages,
		Stream:         true,
		MaxTokens:      s.maxTokens,
		ResponseFormat: responseFormat,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI request body: %w", err)
	}

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodPost, s.baseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("failed to create HTTP request for OpenAI: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream, application/json")
		req.Header.Set("User-Agent", "curl/8.5.0")
		if s.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+s.apiKey)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to call OpenAI API after %d attempts: %w", maxAttempts, err)
			}
			time.Sleep(time.Duration(attempt) * 1500 * time.Millisecond)
			continue
		}

		respStatusCode := resp.StatusCode
		if respStatusCode == 503 || respStatusCode == 529 || respStatusCode == 429 || respStatusCode == 524 {
			resp.Body.Close()
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2000 * time.Millisecond)
				continue
			}
		}

		if respStatusCode != http.StatusOK {
			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			trimmed := strings.TrimSpace(string(respBytes))
			if respStatusCode == http.StatusGatewayTimeout || respStatusCode == http.StatusBadGateway || respStatusCode == 524 || strings.HasPrefix(trimmed, "<") {
				return "", fmt.Errorf("AI router gateway timeout (HTTP %d from %s): model '%s' took too long or proxy timed out: %s", respStatusCode, s.baseURL, s.model, trimmed)
			}
			var errResp dtos.ChatResponseOpenAI
			if err := json.Unmarshal(respBytes, &errResp); err == nil && errResp.Error != nil && errResp.Error.Message != "" {
				return "", fmt.Errorf("OpenAI API error (HTTP %d): %s", respStatusCode, errResp.Error.Message)
			}
			return "", fmt.Errorf("OpenAI API returned status %d: %s", respStatusCode, trimmed)
		}

		// Handle SSE Streaming (keeps connection alive, avoids Cloudflare 524 timeout)
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "text/event-stream") {
			scanner := bufio.NewScanner(resp.Body)
			buf := make([]byte, 1024*1024)
			scanner.Buffer(buf, 10*1024*1024)

			var sb strings.Builder
			var lastFinishReason string
			var completionTokens int
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, ":") {
					continue
				}
				if line == "data: [DONE]" {
					break
				}
				if strings.HasPrefix(line, "data: ") {
					dataPayload := strings.TrimPrefix(line, "data: ")
					var chunk chatCompletionChunk
					if err := json.Unmarshal([]byte(dataPayload), &chunk); err == nil {
						if len(chunk.Choices) > 0 {
							delta := chunk.Choices[0].Delta.Content
							sb.WriteString(delta)
							if onChunk != nil && delta != "" {
								onChunk(delta, completionTokens)
							}
							if chunk.Choices[0].FinishReason != "" {
								lastFinishReason = chunk.Choices[0].FinishReason
							}
						}
						if chunk.Usage != nil {
							completionTokens = chunk.Usage.CompletionTokens
						}
					}
				}
			}
			resp.Body.Close()

			rawText := sb.String()
			log.Printf("[AIService] Streamed response completed: length=%d chars, completion_tokens=%d, finish_reason=%q",
				len(rawText), completionTokens, lastFinishReason)
			if lastFinishReason == "length" {
				log.Printf("[AIService] WARNING: LLM output was truncated by length limit (max_tokens=%d)!", s.maxTokens)
			}
			if strings.TrimSpace(rawText) == "" {
				return "", fmt.Errorf("empty stream content received from OpenAI API")
			}
			return rawText, nil
		}

		// Fallback for standard non-streaming response
		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to read response body: %w", err)
			}
			continue
		}

		var openAIResp dtos.ChatResponseOpenAI
		if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
			return "", fmt.Errorf("failed to unmarshal OpenAI response: %w: %s", err, string(respBytes))
		}
		if len(openAIResp.Choices) == 0 {
			return "", fmt.Errorf("empty choices in OpenAI response: %s", string(respBytes))
		}

		choice := openAIResp.Choices[0]
		log.Printf("[AIService] Token usage: prompt=%d, completion=%d, total=%d, finish_reason=%q (requested_max=%d)",
			openAIResp.Usage.PromptTokens, openAIResp.Usage.CompletionTokens, openAIResp.Usage.TotalTokens, choice.FinishReason, s.maxTokens)
		if choice.FinishReason == "length" {
			log.Printf("[AIService] WARNING: LLM output was truncated by length limit (completion_tokens=%d, max_tokens=%d)!", openAIResp.Usage.CompletionTokens, s.maxTokens)
		}

		rawText := choice.Message.Content
		if strings.TrimSpace(rawText) == "" {
			return "", fmt.Errorf("no content in OpenAI response: %s", string(respBytes))
		}
		return rawText, nil
	}

	return "", fmt.Errorf("failed to call OpenAI API after %d attempts", maxAttempts)
}

func (s *AIService) callAnthropic(
	systemPrompt string,
	historyMessages []entities.ChatMessage,
	newPrompt string,
) (string, error) {
	rawMessages := make([]map[string]string, 0, len(historyMessages)+1)
	for _, msg := range historyMessages {
		role := msg.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		rawMessages = append(rawMessages, map[string]string{
			"role":    role,
			"content": msg.Content,
		})
	}

	if strings.TrimSpace(newPrompt) != "" {
		rawMessages = append(rawMessages, map[string]string{
			"role":    "user",
			"content": strings.TrimSpace(newPrompt),
		})
	}

	var consolidatedMessages []map[string]string
	for _, m := range rawMessages {
		if len(consolidatedMessages) > 0 && consolidatedMessages[len(consolidatedMessages)-1]["role"] == m["role"] {
			consolidatedMessages[len(consolidatedMessages)-1]["content"] += "\n" + m["content"]
		} else {
			consolidatedMessages = append(consolidatedMessages, map[string]string{
				"role":    m["role"],
				"content": m["content"],
			})
		}
	}

	if len(consolidatedMessages) == 0 {
		consolidatedMessages = append(consolidatedMessages, map[string]string{
			"role":    "user",
			"content": "Generate diagram",
		})
	} else if consolidatedMessages[0]["role"] != "user" {
		consolidatedMessages = append([]map[string]string{{"role": "user", "content": "Start conversation"}}, consolidatedMessages...)
	}

	reqBody := anthropicRequest{
		Model:     s.model,
		MaxTokens: s.maxTokens,
		System:    systemPrompt,
		Messages:  consolidatedMessages,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Anthropic request body: %w", err)
	}

	var respBytes []byte
	var respStatusCode int
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodPost, s.baseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("failed to create HTTP request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", s.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := s.client.Do(req)
		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to call Anthropic API after %d attempts: %w", maxAttempts, err)
			}
			time.Sleep(time.Duration(attempt) * 1500 * time.Millisecond)
			continue
		}

		respBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to read response body: %w", err)
			}
			time.Sleep(time.Duration(attempt) * 1500 * time.Millisecond)
			continue
		}

		respStatusCode = resp.StatusCode
		if respStatusCode == 503 || respStatusCode == 529 || strings.Contains(string(respBytes), "No available accounts") {
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2000 * time.Millisecond)
				continue
			}
		}

		break
	}

	if respStatusCode != http.StatusOK {
		return "", fmt.Errorf("Anthropic API returned status %d: %s", respStatusCode, string(respBytes))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBytes, &anthropicResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal Anthropic response: %w", err)
	}

	if anthropicResp.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s - %s", anthropicResp.Error.Type, anthropicResp.Error.Message)
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic API: %s", string(respBytes))
	}

	var rawText string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			rawText += c.Text
		}
	}

	if rawText == "" {
		for _, c := range anthropicResp.Content {
			if c.Text != "" {
				rawText += c.Text
			}
		}
	}

	if rawText == "" {
		return "", fmt.Errorf("no text content found in Anthropic response: %s", string(respBytes))
	}

	return rawText, nil
}

func (s *AIService) CallLLM(systemPrompt string, historyMessages []entities.ChatMessage, newPrompt string) (string, error) {
	if s.provider == "openai" {
		return s.callOpenAI(systemPrompt, historyMessages, newPrompt, &dtos.ResponseFormatOpenAI{Type: "json_object"})
	}
	return s.callAnthropic(systemPrompt, historyMessages, newPrompt)
}

func (s *AIService) CallLLMStream(
	systemPrompt string,
	historyMessages []entities.ChatMessage,
	newPrompt string,
	onChunk LLMStreamCallback,
) (string, error) {
	if s.provider == "openai" {
		return s.callOpenAIWithCallback(systemPrompt, historyMessages, newPrompt, &dtos.ResponseFormatOpenAI{Type: "json_object"}, onChunk)
	}
	return s.callAnthropic(systemPrompt, historyMessages, newPrompt)
}

func (s *AIService) CallLLMText(systemPrompt string, historyMessages []entities.ChatMessage, newPrompt string) (string, error) {
	if s.provider == "openai" {
		return s.callOpenAI(systemPrompt, historyMessages, newPrompt, nil)
	}
	return s.callAnthropic(systemPrompt, historyMessages, newPrompt)
}

func (s *AIService) SanitizeJSON(raw string) string {
	return sanitizeJSONResponse(raw)
}

var (
	codeBlockRegex     = regexp.MustCompile("(?s)```(?:json)?\\s*\n?(.*?)\\s*```")
	trailingCommaRegex = regexp.MustCompile(`,(\s*[}\]])`)
)

func sanitizeJSONResponse(raw string) string {
	trimmed := strings.TrimSpace(raw)
	// Remove UTF-8 BOM if present
	trimmed = strings.TrimPrefix(trimmed, "\xef\xbb\xbf")

	if matches := codeBlockRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
		trimmed = strings.TrimSpace(matches[1])
	} else {
		if strings.HasPrefix(trimmed, "```json") {
			trimmed = strings.TrimPrefix(trimmed, "```json")
		} else if strings.HasPrefix(trimmed, "```") {
			trimmed = strings.TrimPrefix(trimmed, "```")
		}
		if strings.HasSuffix(trimmed, "```") {
			trimmed = strings.TrimSuffix(trimmed, "```")
		}
		trimmed = strings.TrimSpace(trimmed)
	}

	// Find the start of the JSON object or array
	startObj := strings.Index(trimmed, "{")
	startArr := strings.Index(trimmed, "[")
	start := -1
	if startObj != -1 && (startArr == -1 || startObj < startArr) {
		start = startObj
	} else if startArr != -1 {
		start = startArr
	}

	if start == -1 {
		return trimmed
	}
	trimmed = trimmed[start:]

	// Scan through JSON tracking depth, strings, escaped characters, and escape literal newlines in strings
	var stack []byte
	var out strings.Builder
	out.Grow(len(trimmed) + 32)
	inString := false
	escaped := false
	rootClosed := false

	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]

		if rootClosed {
			break
		}

		if escaped {
			escaped = false
			out.WriteByte(ch)
			continue
		}

		if ch == '\\' {
			if inString {
				escaped = true
			}
			out.WriteByte(ch)
			continue
		}

		if ch == '"' {
			inString = !inString
			out.WriteByte(ch)
			continue
		}

		if inString {
			// Escape unescaped control characters inside JSON strings
			if ch == '\n' {
				out.WriteString(`\n`)
			} else if ch == '\r' {
				out.WriteString(`\r`)
			} else if ch == '\t' {
				out.WriteString(`\t`)
			} else {
				out.WriteByte(ch)
			}
			continue
		}

		out.WriteByte(ch)
		if ch == '{' || ch == '[' {
			stack = append(stack, ch)
		} else if ch == '}' || ch == ']' {
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				if (top == '{' && ch == '}') || (top == '[' && ch == ']') {
					stack = stack[:len(stack)-1]
					if len(stack) == 0 {
						rootClosed = true
					}
				}
			}
		}
	}

	// Auto-repair if truncated before root was closed
	if !rootClosed {
		if inString {
			out.WriteByte('"')
		}
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i] == '{' {
				out.WriteByte('}')
			} else if stack[i] == '[' {
				out.WriteByte(']')
			}
		}
	}

	res := out.String()
	// Remove trailing commas before closing curly braces or brackets
	res = trailingCommaRegex.ReplaceAllString(res, "$1")

	return res
}
