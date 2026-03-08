package agent

import (
	pb "cloud.google.com/go/ai/generativelanguage/apiv1beta/generativelanguagepb"
	"github.com/google/generative-ai-go/genai"
)

func convertToolsToJSON(tools []*genai.Tool) []map[string]interface{} {
	var finalResult []map[string]interface{}
	for _, t := range tools {
		if len(t.FunctionDeclarations) > 0 {
			var funcs []map[string]interface{}
			for _, fd := range t.FunctionDeclarations {
				funcMap := map[string]interface{}{
					"name":        fd.Name,
					"description": fd.Description,
				}
				if fd.Parameters != nil {
					funcMap["parameters"] = convertSchemaToJSON(fd.Parameters)
				}
				funcs = append(funcs, funcMap)
			}
			finalResult = append(finalResult, map[string]interface{}{
				"function_declarations": funcs,
			})
		}
	}
	return finalResult
}

func convertSchemaToJSON(s *genai.Schema) map[string]interface{} {
	if s == nil {
		return nil
	}

	// Use the protobuf Type enum's String() method to get the correct API representation (e.g., "STRING", "OBJECT")
	t := pb.Type(s.Type).String()

	m := map[string]interface{}{
		"type": t,
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if len(s.Properties) > 0 {
		props := make(map[string]interface{})
		for k, v := range s.Properties {
			props[k] = convertSchemaToJSON(v)
		}
		m["properties"] = props
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	if s.Items != nil {
		m["items"] = convertSchemaToJSON(s.Items)
	}
	return m
}
