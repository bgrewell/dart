package config

import (
	"gopkg.in/yaml.v3"
)

// extractLocations performs a second parse of the raw YAML data into a
// yaml.Node tree to extract line numbers, then populates the SourceLocation
// fields on the already-unmarshaled Configuration structs. The position
// matching is index-based: the Nth item in the yaml.Node sequence corresponds
// to the Nth item in the struct slice.
func extractLocations(data []byte, filePath string, cfg *Configuration) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return
	}

	// The root document node wraps the top-level mapping
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return
	}
	topMap := root.Content[0]
	if topMap.Kind != yaml.MappingNode {
		return
	}

	// Walk key/value pairs in the top-level mapping
	for i := 0; i+1 < len(topMap.Content); i += 2 {
		keyNode := topMap.Content[i]
		valNode := topMap.Content[i+1]

		switch keyNode.Value {
		case "nodes":
			populateNodeLocations(valNode, filePath, cfg.Nodes)
		case "tests":
			populateTestLocations(valNode, filePath, cfg.Tests)
		case "setup":
			populateStepLocations(valNode, filePath, cfg.Setup)
		case "teardown":
			populateStepLocations(valNode, filePath, cfg.Teardown)
		}
	}
}

// populateNodeLocations sets Loc and TypeLoc on each NodeConfig by matching
// sequence index.
func populateNodeLocations(seq *yaml.Node, filePath string, nodes []*NodeConfig) {
	if seq.Kind != yaml.SequenceNode {
		return
	}
	for idx, itemNode := range seq.Content {
		if idx >= len(nodes) {
			break
		}
		nodes[idx].Loc = SourceLocation{File: filePath, Line: itemNode.Line, Column: itemNode.Column}

		if itemNode.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(itemNode.Content); j += 2 {
				valNode := itemNode.Content[j+1]
				switch itemNode.Content[j].Value {
				case "type":
					nodes[idx].TypeLoc = SourceLocation{File: filePath, Line: valNode.Line, Column: valNode.Column}
				case "options":
					nodes[idx].OptionLocs = optionLocations(valNode, filePath)
				}
			}
		}
	}
}

// populateTestLocations sets Loc, NodeLoc, and TypeLoc on each TestConfig.
func populateTestLocations(seq *yaml.Node, filePath string, tests []*TestConfig) {
	if seq.Kind != yaml.SequenceNode {
		return
	}
	for idx, itemNode := range seq.Content {
		if idx >= len(tests) {
			break
		}
		tests[idx].Loc = SourceLocation{File: filePath, Line: itemNode.Line, Column: itemNode.Column}

		if itemNode.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(itemNode.Content); j += 2 {
				key := itemNode.Content[j].Value
				valNode := itemNode.Content[j+1]
				switch key {
				case "node":
					tests[idx].NodeLoc = SourceLocation{File: filePath, Line: valNode.Line, Column: valNode.Column}
				case "type":
					tests[idx].TypeLoc = SourceLocation{File: filePath, Line: valNode.Line, Column: valNode.Column}
				case "options":
					tests[idx].OptionLocs = optionLocations(valNode, filePath)
				}
			}
		}
	}
}

// optionLocations maps each key of an options mapping to the position of the
// key itself. Pointing at the key rather than the enclosing block is what
// lets an error about `timout:` mark the line `timout:` is written on.
func optionLocations(optionsNode *yaml.Node, filePath string) map[string]SourceLocation {
	if optionsNode == nil || optionsNode.Kind != yaml.MappingNode {
		return nil
	}
	locs := make(map[string]SourceLocation, len(optionsNode.Content)/2)
	for i := 0; i+1 < len(optionsNode.Content); i += 2 {
		keyNode := optionsNode.Content[i]
		locs[keyNode.Value] = SourceLocation{File: filePath, Line: keyNode.Line, Column: keyNode.Column}
	}
	return locs
}

// populateStepLocations sets Loc, NodeLoc, and step.TypeLoc on each StepConfig.
func populateStepLocations(seq *yaml.Node, filePath string, steps []*StepConfig) {
	if seq.Kind != yaml.SequenceNode {
		return
	}
	for idx, itemNode := range seq.Content {
		if idx >= len(steps) {
			break
		}
		steps[idx].Loc = SourceLocation{File: filePath, Line: itemNode.Line, Column: itemNode.Column}

		if itemNode.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(itemNode.Content); j += 2 {
				key := itemNode.Content[j].Value
				valNode := itemNode.Content[j+1]
				switch key {
				case "node":
					steps[idx].NodeLoc = SourceLocation{File: filePath, Line: valNode.Line, Column: valNode.Column}
				case "step":
					// The "step" value is itself a mapping; find "type" inside it
					if valNode.Kind == yaml.MappingNode {
						for k := 0; k+1 < len(valNode.Content); k += 2 {
							switch valNode.Content[k].Value {
							case "type":
								steps[idx].Step.TypeLoc = SourceLocation{
									File:   filePath,
									Line:   valNode.Content[k+1].Line,
									Column: valNode.Content[k+1].Column,
								}
							case "options":
								steps[idx].OptionLocs = optionLocations(valNode.Content[k+1], filePath)
							}
						}
					}
				}
			}
		}
	}
}
