package qfs

// Link is a reference to a foreign resource
type Link struct {
	Ref   string      `json:"@id"`
	Value interface{} `json:"value,omitempty"`
}

// GatherDereferencedLinksAsFileValues TODO(b5)
func GatherDereferencedLinksAsFileValues(data interface{}) ([]*Link, error) {
	links := []*Link{}
	err := gatherLinks(data, &links)
	return links, err
}

func gatherLinks(data interface{}, links *[]*Link) error {
	switch x := data.(type) {
	case *Link:
		if x.Value != nil {
			f, err := valueToFile(x.Ref, x.Value)
			if err != nil {
				return err
			}
			x.Value = f
			*links = append(*links, x)
		}
	case map[string]interface{}:
		for _, val := range x {
			if err := gatherLinks(val, links); err != nil {
				return err
			}
		}
	case map[interface{}]interface{}:
		for _, val := range x {
			if err := gatherLinks(val, links); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, val := range x {
			if err := gatherLinks(val, links); err != nil {
				return err
			}
		}
	// default:
	// 	return fmt.Errorf("unrecognized linked data data type: %T", data)
	}
	return nil
}

func valueToFile(name string, val interface{}) (File, error) {
	if f, ok := val.(File); ok {
		return f, nil
	}
	return NewLinkedDataFile(name, val)
}