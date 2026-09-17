package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/config"
	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type AIService struct {
	client   *http.Client
	provider string
	apiKey   string
	model    string
	baseURL  string
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

	return &AIService{
		client: &http.Client{
			Timeout: 180 * time.Second,
		},
		provider: provider,
		apiKey:   apiKey,
		model:    model,
		baseURL:  baseURL,
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

7. MINDMAP / CONCEPT HIERARCHY / NETWORK TOPOLOGY:
   - "input": Root concept, main topic, or Public Internet.
   - "decision": Routers, Firewalls, or major decision points.
   - "default": Branches, subtopics, internal subnets, or team departments.
   - "output": Leaves, action items, or secured internal servers.

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
		case "state", "lifecycle":
			systemPrompt += "\n- MANDATORY: Design state machine/lifecycle transitions. Nodes represent states (Draft, Active, Finished) and edges represent transition events/triggers."
		case "uml", "class":
			systemPrompt += "\n- MANDATORY: Design UML class diagram. Use type: 'database' where columns list attributes and methods."
		case "pipeline", "dfd":
			systemPrompt += "\n- MANDATORY: Design data pipeline / ETL data flow. Use 'input' (data sources), 'default' (transformations/stream processing), 'database' (lake/warehouse), and 'output' (analytics/dashboards)."
		case "mindmap", "network":
			systemPrompt += "\n- MANDATORY: Design concept mindmap or network topology. Use 'input' (root concept or core gateway), 'decision' (major branch points or routers), 'default' (subtopics or subnet nodes), and 'output' (leaves or endpoints)."
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

func (s *AIService) callOpenAI(
	systemPrompt string,
	historyMessages []entities.ChatMessage,
	newPrompt string,
	responseFormat *dtos.ResponseFormatOpenAI,
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

	reqBody := dtos.ChatRequestOpenAI{
		Model:          s.model,
		Messages:       openAIMessages,
		Stream:         false,
		MaxTokens:      8192,
		ResponseFormat: responseFormat,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI request body: %w", err)
	}

	var respBytes []byte
	var respStatusCode int
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodPost, s.baseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("failed to create HTTP request for OpenAI: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
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
		if respStatusCode == 503 || respStatusCode == 529 || respStatusCode == 429 {
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2000 * time.Millisecond)
				continue
			}
		}

		break
	}

	var openAIResp dtos.ChatResponseOpenAI
	if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal OpenAI response (HTTP %d): %w: %s", respStatusCode, err, string(respBytes))
	}

	if openAIResp.Error != nil && openAIResp.Error.Message != "" {
		return "", fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	if respStatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API returned status %d: %s", respStatusCode, string(respBytes))
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices in OpenAI response: %s", string(respBytes))
	}

	rawText := openAIResp.Choices[0].Message.Content
	if strings.TrimSpace(rawText) == "" {
		return "", fmt.Errorf("no content in OpenAI response: %s", string(respBytes))
	}

	return rawText, nil
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
		MaxTokens: 8192,
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

func (s *AIService) CallLLMText(systemPrompt string, historyMessages []entities.ChatMessage, newPrompt string) (string, error) {
	if s.provider == "openai" {
		return s.callOpenAI(systemPrompt, historyMessages, newPrompt, nil)
	}
	return s.callAnthropic(systemPrompt, historyMessages, newPrompt)
}

func (s *AIService) SanitizeJSON(raw string) string {
	return sanitizeJSONResponse(raw)
}

var codeBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*\n?(.*?)\\s*```")

func sanitizeJSONResponse(raw string) string {
	trimmed := strings.TrimSpace(raw)

	if matches := codeBlockRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(trimmed[start : end+1])
	}

	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}
