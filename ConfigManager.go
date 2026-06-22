package webtools

type ConfigManager struct {
	path    string
	options *SafeMap[string, ConfigManagerOption]
	values  *SafeMap[string, any]
}

type ConfigManagerOption struct {
	DefaultValue any
	Description  string
}

func (configManager *ConfigManager) Save() {
	configManager.SaveAs(configManager.path)
}

func (configManager *ConfigManager) SaveAs(path string) {
	
}

/*
JSONCToJSON converts JSONC (commented JSON) to JSON. It destroyes all comments
*/
func JSONCToJSON(jsonc []byte) []byte {
	isInStarComment := false
	isInInlineCommment := false
	isInString := false
	result := make([]byte, 0)
	for i := range jsonc {
		// Get previous char
		var prev byte
		if i > 0 {
			prev = jsonc[i-1]
		}

		// Sort commnet cases
		if prev != '\\' && jsonc[i] == '"' && !isInInlineCommment && !isInStarComment {
			isInString = !isInString
		}
		if prev == '/' && jsonc[i] == '*' && !isInString && !isInInlineCommment {
			isInStarComment = true
		}
		if prev == '*' && jsonc[i] == '/' && !isInString && !isInInlineCommment {
			isInStarComment = false
		}
		if !isInStarComment && !isInString && prev == '/' && jsonc[i] == '/' {
			isInInlineCommment = true
		}
		if isInInlineCommment && jsonc[i] == '\n' {
			isInInlineCommment = false
		}

		// Write to buffer
		if jsonc[i] == '\n' {
			result = append(result, '\n')
		} else if isInInlineCommment || isInStarComment {
			result = append(result, ' ')
		} else {
			result = append(result, jsonc[i])
		}
	}
	return result
}
