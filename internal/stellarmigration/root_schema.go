package stellarmigration

import (
	"fmt"
	"strings"
	"unicode"
)

// 源sqlite_master也需单语句校验；不允许损坏/篡改schema把ATTACH等语句带入演练。
func safeDatabaseObjectStatement(object DatabaseObject) error {
	tokens, err := schemaTokens(object.SQL)
	if err != nil {
		return err
	}
	if len(tokens) < 3 || tokens[0] != "CREATE" {
		return fmt.Errorf("invalid captured schema declaration")
	}
	at := 1
	if object.Kind == "index" && tokens[at] == "UNIQUE" {
		at++
	}
	if tokens[at] != strings.ToUpper(object.Kind) {
		return fmt.Errorf("captured schema object kind changed")
	}
	if object.Kind != "trigger" {
		return safeTableStatement("CREATE TABLE " + object.SQL)
	}
	inBody := false
	cases := 0
	finished := false
	for _, token := range tokens[at+1:] {
		if finished {
			if token != ";" {
				return fmt.Errorf("multiple captured schema statements")
			}
			continue
		}
		if !inBody {
			if token == ";" {
				return fmt.Errorf("unexpected trigger declaration terminator")
			}
			if token == "BEGIN" {
				inBody = true
			}
			continue
		}
		if token == "CASE" {
			cases++
		}
		if token == "END" {
			if cases > 0 {
				cases--
			} else {
				finished = true
			}
		}
	}
	if !finished || cases != 0 {
		return fmt.Errorf("unclosed captured trigger body")
	}
	return nil
}
func schemaTokens(s string) ([]string, error) {
	var tokens []string
	for i := 0; i < len(s); {
		if unicode.IsSpace(rune(s[i])) {
			i++
			continue
		}
		if i+1 < len(s) && s[i:i+2] == "--" {
			end := strings.IndexByte(s[i+2:], '\n')
			if end < 0 {
				break
			}
			i += end + 3
			continue
		}
		if i+1 < len(s) && s[i:i+2] == "/*" {
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("unclosed schema comment")
			}
			i += end + 4
			continue
		}
		ch := s[i]
		if ch == '\'' || ch == '"' || ch == '`' || ch == '[' {
			end := ch
			if ch == '[' {
				end = ']'
			}
			i++
			closed := false
			for i < len(s) {
				if s[i] == end {
					if end != ']' && i+1 < len(s) && s[i+1] == end {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unclosed schema token")
			}
			tokens = append(tokens, "QUOTED")
			continue
		}
		if ch == ';' {
			tokens = append(tokens, ";")
			i++
			continue
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
			start := i
			i++
			for i < len(s) && ((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= '0' && s[i] <= '9') || s[i] == '_') {
				i++
			}
			tokens = append(tokens, strings.ToUpper(s[start:i]))
			continue
		}
		i++
	}
	return tokens, nil
}
