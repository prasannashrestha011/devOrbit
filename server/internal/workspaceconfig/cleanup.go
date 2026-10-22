package workspaceconfig

import "gopkg.in/yaml.v3"

// RemoveServicePorts discards obsolete configuration before strict service loading.
func RemoveServicePorts(root string, snapshot Snapshot) error {
	document, err := parseDocument([]byte(snapshot.Content))
	if err != nil {
		return err
	}
	changed := false
	fields := document.Content[0].Content
	for i := 0; i < len(fields); i += 2 {
		if fields[i].Value != "services" || fields[i+1].Kind != yaml.MappingNode {
			continue
		}
		services := fields[i+1].Content
		for j := 1; j < len(services); j += 2 {
			service := services[j]
			if service.Kind != yaml.MappingNode {
				continue
			}
			for k := 0; k < len(service.Content); k += 2 {
				if service.Content[k].Value == "ports" {
					service.Content = append(service.Content[:k], service.Content[k+2:]...)
					changed = true
					break
				}
			}
		}
	}
	if !changed {
		return nil
	}
	data, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	_, err = Write(root, data, snapshot.Revision)
	return err
}
