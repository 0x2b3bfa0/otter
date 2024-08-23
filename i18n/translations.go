package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/a-h/templ"
	"github.com/martinmunillas/otter/utils"
)

// https://github.com/opral/monorepo/blob/main/inlang/source-code/plugins/t-function-matcher/marketplace-manifest.json
func flattenJson(input map[string]interface{}) (map[string]string, error) {
	flatMap := make(map[string]string)

	var flatten func(map[string]interface{}, string) error
	flatten = func(data map[string]interface{}, prefix string) error {
		for key, value := range data {
			fullKey := key
			if prefix != "" {
				fullKey = prefix + "." + key
			}

			// Type switch to handle nested maps
			switch v := value.(type) {
			case map[string]interface{}:
				err := flatten(v, fullKey)
				if err != nil {
					return err
				}
			case string:
				flatMap[fullKey] = v
			default:
				return fmt.Errorf("invalid translation %s of type %t", key, v)
			}
		}
		return nil
	}

	err := flatten(input, "")
	return flatMap, err
}

var translations = make(map[string]map[string]string, 2)
var supportedLocales = make([]string, 0, 2)

func processLang(r io.Reader) (map[string]string, error) {
	m := map[string]interface{}{}
	err := json.NewDecoder(r).Decode(&m)
	if err != nil {
		return nil, err
	}
	translation, err := flattenJson(m)
	if err != nil {
		return nil, err
	}

	return translation, nil

}
func AddLocale(locale string, r io.Reader) {
	translation, err := processLang(r)
	if err != nil {
		utils.Throw(err.Error())
	}
	supportedLocales = append(supportedLocales, locale)
	translations[locale] = translation
	if defaultLocale == "" {
		defaultLocale = locale
	}

}

func t(ctx context.Context, strChunk func(string, ...error) templ.Component, key string, replacements ...map[string]any) templ.Component {
	str := Translation(ctx, key)
	if len(replacements) == 0 || str == key {
		return strChunk(str)
	}
	if len(replacements) > 1 {
		return strChunk(fmt.Sprintf("Invalid message \"%s\" call: more than one replacements map provided", key))
	}
	runes := []rune(str)

	chunks := make([]templ.Component, 0, 1)

	currentStr := ""
	currentVarName := ""
	isCollectingVarName := false
	for i, c := range runes {
		isEscaped := i > 0 && runes[i-1] == '\\'
		if c == '{' && !isEscaped {
			if isCollectingVarName {
				return strChunk(fmt.Sprintf("Invalid message \"%s\" format: opening variable before closing previous", key))
			}
			if currentStr != "" {
				chunks = append(chunks, strChunk(currentStr))
				currentStr = ""
			}
			isCollectingVarName = true
			continue
		}
		if c == '}' && !isEscaped {
			if !isCollectingVarName {
				return strChunk(fmt.Sprintf("Invalid message \"%s\" format: closing variable before opening one", key))
			}
			if currentVarName == "" {
				return strChunk(fmt.Sprintf("Invalid message \"%s\" format: missing variable name between {}", key))
			}
			val, ok := replacements[0][currentVarName]
			if !ok {
				return strChunk(fmt.Sprintf("Invalid message \"%s\" call: missing variable \"%s\" value", key, currentVarName))
			}
			switch v := val.(type) {
			case templ.Component:
				chunks = append(chunks, v)
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				chunks = append(chunks, strChunk(fmt.Sprintf("%d", v)))
			case string, []rune, []byte:
				chunks = append(chunks, strChunk(fmt.Sprintf("%s", v)))
			default:
				return strChunk(fmt.Sprintf("Invalid message \"%s\" call: variable \"%s\" of type %t not supported", key, currentVarName, v))

			}
			currentVarName = ""
			isCollectingVarName = false
			continue
		}

		if isCollectingVarName {
			currentVarName += string(c)
		} else {
			currentStr += string(c)
		}
	}
	if currentStr != "" {
		chunks = append(chunks, strChunk(currentStr))
	}
	return chunksRender(chunks)
}

func T(ctx context.Context, key string, replacements ...map[string]any) templ.Component {
	return t(ctx, strChunk, key, replacements...)
}

func RawT(ctx context.Context, key string, replacements ...map[string]any) templ.Component {
	return t(ctx, templ.Raw, key, replacements...)
}

func Translation(ctx context.Context, key string) string {
	locale := FromCtx(ctx)
	content := translations[locale][key]
	if content == "" {
		return key
	}
	return content
}
