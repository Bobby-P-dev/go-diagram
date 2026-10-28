package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"log"
	"strings"
)

func (s *uiDesignService) IterateUIDesignWithChat(ctx context.Context, projectID, prompt string, targetedNodeIDs []string) (*dtos.ProjectResponse, error) {
	return s.IterateUIDesignWithTargetedChat(ctx, projectID, &dtos.ChatRequest{
		Prompt:          prompt,
		TargetedNodeIDs: targetedNodeIDs,
	})
}

func (s *uiDesignService) IterateUIDesignWithTargetedChat(ctx context.Context, projectID string, req *dtos.ChatRequest) (*dtos.ProjectResponse, error) {
	userID := s.userID(ctx)

	project, messages, err := s.projects.GetProjectScoped(ctx, projectID, userID, "")
	if err != nil {
		return nil, fmt.Errorf("project not found or unauthorized: %w", err)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(req.Message)
	}
	if prompt == "" {
		prompt = strings.TrimSpace(req.Instruction)
	}

	targetRef := req.Target
	if targetRef == nil && req.SelectedComponentID != "" {
		targetRef = &dtos.TargetElementRefDTO{
			Type: "component",
			ID:   req.SelectedComponentID,
		}
	}

	// 1. Parse existing nodes and find current primary UI Frame
	var currentNodes []map[string]interface{}
	_ = json.Unmarshal(project.CurrentNodes, &currentNodes)

	var currentFrame *dtos.UIFrameData
	var targetNodeIdx = -1

	// 1.1 Match by explicit TargetedNodeIDs from request
	if len(req.TargetedNodeIDs) > 0 {
		targetID := req.TargetedNodeIDs[0]
		for idx, n := range currentNodes {
			if n["type"] == "ui_frame" && fmt.Sprintf("%v", n["id"]) == targetID {
				if dataMap, ok := n["data"].(map[string]interface{}); ok {
					dataBytes, _ := json.Marshal(dataMap)
					var f dtos.UIFrameData
					if err := json.Unmarshal(dataBytes, &f); err == nil {
						currentFrame = &f
						targetNodeIdx = idx
						break
					}
				}
			}
		}
	}

	// 1.2 Match by Target.FrameID if provided
	if targetNodeIdx == -1 && targetRef != nil && targetRef.FrameID != "" {
		for idx, n := range currentNodes {
			if n["type"] == "ui_frame" && fmt.Sprintf("%v", n["id"]) == targetRef.FrameID {
				if dataMap, ok := n["data"].(map[string]interface{}); ok {
					dataBytes, _ := json.Marshal(dataMap)
					var f dtos.UIFrameData
					if err := json.Unmarshal(dataBytes, &f); err == nil {
						currentFrame = &f
						targetNodeIdx = idx
						break
					}
				}
			}
		}
	}

	// 1.3 If user asks to switch to web (e.g. "ubah dari mobile ke bentuk web"), prioritize targeting a mobile frame
	lowerPrompt := strings.ToLower(prompt)
	isSwitchingToWeb := strings.Contains(lowerPrompt, "web") && (strings.Contains(lowerPrompt, "mobile") || strings.Contains(lowerPrompt, "hp"))
	if targetNodeIdx == -1 && isSwitchingToWeb {
		for idx, n := range currentNodes {
			if n["type"] == "ui_frame" {
				if dataMap, ok := n["data"].(map[string]interface{}); ok {
					if dev, ok := dataMap["device"].(string); ok && (dev == "mobile" || dev == "smartphone") {
						dataBytes, _ := json.Marshal(dataMap)
						var f dtos.UIFrameData
						if err := json.Unmarshal(dataBytes, &f); err == nil {
							currentFrame = &f
							targetNodeIdx = idx
							break
						}
					}
				}
			}
		}
	}

	// 1.4 Fallback: Find the first valid ui_frame
	if targetNodeIdx == -1 {
		for idx, n := range currentNodes {
			if n["type"] == "ui_frame" {
				if dataMap, ok := n["data"].(map[string]interface{}); ok {
					dataBytes, _ := json.Marshal(dataMap)
					var f dtos.UIFrameData
					if err := json.Unmarshal(dataBytes, &f); err == nil {
						currentFrame = &f
						targetNodeIdx = idx
						break
					} else {
						log.Printf("[UI_FRAME UNMARSHAL ERROR] Failed to unmarshal node %d into UIFrameData: %v", idx, err)
					}
				}
			}
		}
	}

	var oldHtml string
	if currentFrame != nil {
		oldHtml = currentFrame.RawHtml
		if oldHtml == "" && currentFrame.CodeExport != nil {
			oldHtml = currentFrame.CodeExport["html"]
		}
		if oldHtml == "" && currentFrame.Implementation != nil {
			oldHtml = currentFrame.Implementation.Source.HTML
		}
	}

	// 2. RUN CHANGE ANALYZER WITH STRICT LOCALITY
	changePlan, err := s.changeAnalyzer.AnalyzeTargetedChange(ctx, prompt, targetRef, req.SelectionContext, currentFrame)
	if err != nil {
		changePlan = &dtos.ChangePlanDTO{
			Request:        prompt,
			Classification: "style",
			Scope:          "component",
			Strategy:       "component_patch",
			Preserve:       []string{"layout", "typography", "content"},
			Regenerate:     false,
		}
	}

	// 3.0 APPEND A NEW FRAME (INSERT_FRAME): add a new page/screen to the project
	// while preserving every existing frame node untouched.
	if changePlan.Operation == dtos.OpInsertFrame {
		return s.appendNewUIFrame(ctx, project, messages, currentNodes, changePlan, prompt)
	}

	// 3.0.1 DEVICE / VIEWPORT MODE SWITCH: Convert frame between mobile and web/desktop
	// while preserving every existing frame node untouched!
	if changePlan.Operation == dtos.OpDeviceModeSwitch || changePlan.Strategy == "device_switch" || changePlan.Target.Type == "device" {
		return s.switchFrameDevice(ctx, project, messages, currentNodes, targetNodeIdx, currentFrame, changePlan, prompt)
	}

	// 3.0.2 DELETE FRAME: Remove a screen from the canvas
	if changePlan.Operation == dtos.OpDeleteFrame || changePlan.Strategy == "delete_frame" {
		return s.deleteFrame(ctx, project, messages, currentNodes, targetNodeIdx, currentFrame, prompt)
	}

	var summary string
	var updatedNodes []interface{}

	// 3. DECIDE: SURGICAL TARGETED PATCH VS REBUILD
	isLocalStrategy := changePlan.Strategy == "component_patch" ||
		changePlan.Strategy == "section_patch" ||
		changePlan.Strategy == "patch" ||
		changePlan.Strategy == "token_update" ||
		changePlan.Strategy == "insert_section" ||
		changePlan.Strategy == "delete_section" ||
		changePlan.Operation == dtos.OpComponentPatch ||
		changePlan.Operation == dtos.OpSectionPatch ||
		changePlan.Operation == dtos.OpInsertSection ||
		changePlan.Operation == dtos.OpDeleteSection

	if isLocalStrategy && currentFrame != nil && targetNodeIdx >= 0 {
		// Attempt surgical targeted patch
		patchedFrame, patchExplanation, patchErr := s.targetedPatcher.ApplyTargetedPatch(ctx, currentFrame, changePlan)
		if patchErr != nil || patchedFrame == nil {
			log.Printf("[TARGETED PATCH FAILED]: %v (falling back to legacy patch engine)", patchErr)
			// Fallback to legacy patch engine if targeted patcher encountered an issue
			patchedFrame, patchExplanation, patchErr = s.patchEngine.ApplyPatch(currentFrame, changePlan)
			if patchErr != nil {
				log.Printf("[LEGACY PATCH ENGINE FAILED]: %v", patchErr)
			}
		}

		if patchErr == nil && patchedFrame != nil {
			patchedFrameBytes, _ := json.Marshal(patchedFrame)
			var patchedMap map[string]interface{}
			_ = json.Unmarshal(patchedFrameBytes, &patchedMap)

			// Ensure raw_html, rawHtml, and implementation.source.html are synchronized
			if rHtml, ok := patchedMap["raw_html"].(string); ok && rHtml != "" {
				patchedMap["rawHtml"] = rHtml
				if implMap, ok := patchedMap["implementation"].(map[string]interface{}); ok {
					if srcMap, ok := implMap["source"].(map[string]interface{}); ok {
						srcMap["html"] = rHtml
					}
				}
			}

			// Preserve existing node position and id
			existingNode := currentNodes[targetNodeIdx]
			existingNode["data"] = patchedMap

			for _, n := range currentNodes {
				updatedNodes = append(updatedNodes, n)
			}

			summary = fmt.Sprintf("✨ **[%s]** %s", strings.ToUpper(changePlan.Strategy), patchExplanation)
		}
	}

	// 4. FULL REBUILD ONLY IF EXPLICITLY REQUESTED
	if len(updatedNodes) == 0 {
		if changePlan.Strategy == "rebuild" || changePlan.Regenerate || changePlan.Operation == dtos.OpFullReplace {
			device := "web"
			foundation := ""
			themeMode := ""
			accentColor := ""

			if changePlan.Target.Device != "" {
				device = changePlan.Target.Device
			} else if currentFrame != nil && currentFrame.Device != "" {
				device = currentFrame.Device
			}

			if currentFrame != nil {
				if mode, ok := currentFrame.Theme["mode"].(string); ok && mode != "" {
					themeMode = mode
				}
				if prim, ok := currentFrame.Theme["primary"].(string); ok && prim != "" {
					accentColor = prim
				}
			}

			dsl, compileErr := s.compiler.Compile(ctx, prompt, device, foundation, themeMode, accentColor)
			if compileErr == nil && len(dsl.Frames) > 0 {
				dsl.ChangePlan = changePlan
				if len(currentNodes) > 1 && targetNodeIdx >= 0 && len(dsl.Frames) == 1 {
					// Replace only the targeted frame, preserve all other frames!
					f := dsl.Frames[0]
					f.ChangePlan = changePlan
					frameBytes, _ := json.Marshal(f)
					var frameMap map[string]interface{}
					_ = json.Unmarshal(frameBytes, &frameMap)
					if rHtml, ok := frameMap["raw_html"].(string); ok && rHtml != "" {
						frameMap["rawHtml"] = rHtml
					}
					for i, n := range currentNodes {
						if i == targetNodeIdx {
							updatedNodes = append(updatedNodes, map[string]interface{}{
								"id":       n["id"],
								"type":     "ui_frame",
								"position": n["position"],
								"data":     frameMap,
							})
						} else {
							updatedNodes = append(updatedNodes, n)
						}
					}
				} else {
					xOffset := 60.0
					for idx, f := range dsl.Frames {
						f.ChangePlan = changePlan
						frameBytes, _ := json.Marshal(f)
						var frameMap map[string]interface{}
						_ = json.Unmarshal(frameBytes, &frameMap)

						if rHtml, ok := frameMap["raw_html"].(string); ok && rHtml != "" {
							frameMap["rawHtml"] = rHtml
						}

						flowNode := map[string]interface{}{
							"id":       fmt.Sprintf("ui-frame-%d", idx+1),
							"type":     "ui_frame",
							"position": map[string]float64{"x": xOffset, "y": 60},
							"data":     frameMap,
						}
						updatedNodes = append(updatedNodes, flowNode)
						xOffset += float64(f.Width) + 80.0
					}
				}
				summary = fmt.Sprintf("✨ **[REBUILD STRUKTURAL]** Desain antarmuka disusun ulang sesuai kebutuhan: %q.", prompt)
			}
		}

		// Safe Locality Guard: If still empty, preserve current layout completely
		if len(updatedNodes) == 0 {
			summary = fmt.Sprintf("⚠️ **[NO_CHANGE]** Tidak ada modifikasi yang diterapkan untuk instruksi: %q. Pastikan elemen target dipilih dengan benar.", prompt)
			for _, n := range currentNodes {
				updatedNodes = append(updatedNodes, n)
			}
		}
	}

	// Validate duplicate IDs across updated nodes
	validator := NewDuplicateValidator()
	for _, n := range updatedNodes {
		if nodeMap, ok := n.(map[string]interface{}); ok {
			if dataMap, ok := nodeMap["data"].(map[string]interface{}); ok {
				if rHtml, ok := dataMap["raw_html"].(string); ok && rHtml != "" {
					if dupErr := validator.ValidateHTMLDuplicateIDs(rHtml); dupErr != nil {
						log.Printf("[VALIDATOR WARNING] %v in targeted chat result", dupErr)
					}
				}
			}
		}
	}

	log.Printf("[STATE RECONCILIATION] op=%s target=%s before_nodes=%d after_nodes=%d", changePlan.Operation, changePlan.Target.ID, len(currentNodes), len(updatedNodes))

	var newHtml string
	for _, n := range updatedNodes {
		if nodeMap, ok := n.(map[string]interface{}); ok {
			if dataMap, ok := nodeMap["data"].(map[string]interface{}); ok {
				if rHtml, ok := dataMap["raw_html"].(string); ok && rHtml != "" {
					newHtml = rHtml
					break
				}
			}
		}
	}

	latestVer, _ := s.projects.GetLatestVersionNumber(ctx, project.ID)
	if latestVer <= 0 {
		latestVer = 1
	}

	// No-Op Detector: Check if targeted edit produced identical markup
	isNoOp := isLocalStrategy && len(currentNodes) == len(updatedNodes) && strings.TrimSpace(newHtml) != "" && strings.TrimSpace(newHtml) == strings.TrimSpace(oldHtml)
	if isNoOp {
		summary = fmt.Sprintf("⚠️ **[NO_CHANGE]** Tidak ada modifikasi kode yang dihasilkan untuk target `%s`. Instruksi tidak mengubah markup yang ada.", changePlan.Target.ID)
	}

	versionNum := latestVer
	if !isNoOp {
		versionNum = latestVer + 1
	}

	// Synchronize version and implementation into node data
	for _, n := range updatedNodes {
		if nodeMap, ok := n.(map[string]interface{}); ok {
			if dataMap, ok := nodeMap["data"].(map[string]interface{}); ok {
				dataMap["version"] = versionNum
				if rHtml, ok := dataMap["raw_html"].(string); ok && rHtml != "" {
					if implMap, ok := dataMap["implementation"].(map[string]interface{}); ok {
						implMap["version"] = versionNum
						if srcMap, ok := implMap["source"].(map[string]interface{}); ok {
							srcMap["html"] = rHtml
						}
					}
				}
			}
		}
	}

	nodesBytes, _ := json.Marshal(updatedNodes)
	edgesBytes := project.CurrentEdges

	if !isNoOp {
		if err := s.projects.UpdateGraph(ctx, project.ID, nodesBytes, edgesBytes); err != nil {
			return nil, fmt.Errorf("failed to update graph: %w", err)
		}
		_ = s.projects.CreateVersion(ctx, uidesign.CreateVersionInput{ProjectID: project.ID, VersionNumber: versionNum, ChangeSummary: summary, Nodes: nodesBytes, Edges: edgesBytes, TriggerMessageID: nil})
	} else {
		log.Printf("[NO-OP DETECTOR] Skipped creating redundant version for project %s: zero diff detected", project.ID)
	}

	// Save messages
	var targetBytes json.RawMessage
	if len(req.TargetedNodeIDs) > 0 {
		targetBytes, _ = json.Marshal(req.TargetedNodeIDs)
	} else if req.Target != nil {
		targetBytes, _ = json.Marshal(req.Target)
	}
	msg1, _ := s.projects.AppendMessage(ctx, project.ID, "user", prompt, targetBytes)
	msg2, _ := s.projects.AppendMessage(ctx, project.ID, "assistant", summary, nil)

	updatedMessages := append(messages, *msg1, *msg2)
	var messageDTOs []dtos.ChatMessageDTO
	for _, m := range updatedMessages {
		messageDTOs = append(messageDTOs, dtos.ChatMessageDTO{
			ID:        m.ID,
			ProjectID: m.ProjectID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return &dtos.ProjectResponse{
		ID:            project.ID,
		Title:         project.Title,
		DiagramType:   project.DiagramType,
		CurrentNodes:  nodesBytes,
		CurrentEdges:  edgesBytes,
		Nodes:         nodesBytes,
		Edges:         edgesBytes,
		Messages:      messageDTOs,
		Version:       versionNum,
		VersionNumber: versionNum,
		CreatedAt:     project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     project.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
