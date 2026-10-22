package workspaceconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const RelativePath = ".devorbit/workspace.yaml"

var ErrInvalid = errors.New("invalid workspace YAML")
var ErrConflict = errors.New("workspace YAML changed; reload before saving")
var writeMu sync.Mutex

type Service struct {
	Command string `yaml:"command" json:"command"`
	CWD     string `yaml:"cwd,omitempty" json:"cwd"`
}

type Config struct {
	Services map[string]Service `yaml:"services" json:"services"`
}

type Snapshot struct {
	Content  string `json:"content"`
	Revision string `json:"revision"`
	Config   Config `json:"config"`
}

func Path(root string) string { return filepath.Join(root, RelativePath) }

func Parse(root string, data []byte) (Config, error) {
	if _, err := parseDocument(data); err != nil {
		return Config{}, err
	}
	var config Config
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if config.Services == nil {
		return Config{}, fmt.Errorf("%w: services must be a mapping", ErrInvalid)
	}
	for key, item := range config.Services {
		if !validServiceKey(key) {
			return Config{}, fmt.Errorf("%w: invalid service key %q", ErrInvalid, key)
		}
		if _, err := SplitCommand(item.Command); err != nil {
			return Config{}, fmt.Errorf("%w: services.%s.command: %v", ErrInvalid, key, err)
		}
		if _, err := ResolveDirectory(root, item.CWD); err != nil {
			return Config{}, fmt.Errorf("%w: services.%s.cwd: %v", ErrInvalid, key, err)
		}
	}
	return config, nil
}

func parseDocument(data []byte) (*yaml.Node, error) {
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&node); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%w: expected one YAML document", ErrInvalid)
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: root must be a mapping", ErrInvalid)
	}
	if err := checkNode(node.Content[0]); err != nil {
		return nil, err
	}
	return &node, nil
}

func validServiceKey(key string) bool {
	if key == "" || key != strings.TrimSpace(key) || utf8.RuneCountInString(key) > 120 {
		return false
	}
	for _, character := range key {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func checkNode(node *yaml.Node) error {
	if node.Anchor != "" || node.Alias != nil || node.Kind == yaml.AliasNode {
		return fmt.Errorf("%w: YAML anchors and aliases are unsupported", ErrInvalid)
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || seen[key.Value] || key.Value == "<<" {
				return fmt.Errorf("%w: duplicate or invalid mapping key %q", ErrInvalid, key.Value)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := checkNode(child); err != nil {
			return err
		}
	}
	return nil
}

func ResolveDirectory(root, directory string) (string, error) {
	if strings.ContainsRune(directory, 0) {
		return "", errors.New("invalid directory path")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", errors.New("workspace root is inaccessible")
	}
	path := directory
	if !filepath.IsAbs(path) {
		path = filepath.Join(canonicalRoot, path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", errors.New("directory does not exist")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("must be a directory")
	}
	if !filepath.IsAbs(directory) {
		rel, err := filepath.Rel(canonicalRoot, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", errors.New("relative directory must remain inside the workspace")
		}
	}
	return resolved, nil
}

// SplitCommand accepts one executable and arguments, without shell evaluation.
func SplitCommand(input string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	started := false
	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == 0 || ch == '\n' || ch == '\r' || ch == '$' || ch == '`' {
			return nil, errors.New("shell expansion and control characters are unsupported")
		}
		if ch == '\\' && quote != '\'' && i+1 < len(runes) && (strings.ContainsRune(" \t\"'\\", runes[i+1])) {
			i++
			word.WriteRune(runes[i])
			started = true
		} else if quote != 0 && ch == quote {
			quote = 0
		} else if quote == 0 && (ch == '\'' || ch == '"') {
			quote = ch
			started = true
		} else if quote == 0 && strings.ContainsRune(" \t", ch) {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
		} else if quote == 0 && strings.ContainsRune("|&;<>", ch) {
			return nil, errors.New("shell operators are unsupported")
		} else {
			word.WriteRune(ch)
			started = true
		}
	}
	if quote != 0 {
		return nil, errors.New("unclosed quote")
	}
	if started {
		words = append(words, word.String())
	}
	if len(words) == 0 || words[0] == "" {
		return nil, errors.New("command must name an executable")
	}
	return words, nil
}

func JoinCommand(words []string) string {
	parts := make([]string, len(words))
	for i, word := range words {
		if word == "" || strings.ContainsAny(word, " \t\"'\\|&;<>$`\n\r") {
			parts[i] = "\"" + strings.ReplaceAll(strings.ReplaceAll(word, "\\", "\\\\"), "\"", "\\\"") + "\""
		} else {
			parts[i] = word
		}
	}
	return strings.Join(parts, " ")
}

func Encode(config Config) ([]byte, error) {
	if config.Services == nil {
		config.Services = map[string]Service{}
	}
	// yaml.v3 sorts map keys, keeping the tracked file stable across UI edits.
	return yaml.Marshal(config)
}

func Read(root string) (Snapshot, error) {
	snapshot, err := ReadRaw(root)
	if err != nil {
		return Snapshot{}, err
	}
	config, err := Parse(root, []byte(snapshot.Content))
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Config = config
	return snapshot, nil
}

func ReadRaw(root string) (Snapshot, error) {
	if info, err := os.Lstat(filepath.Dir(Path(root))); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return Snapshot{}, fmt.Errorf("%w: configuration directory must be a real directory", ErrInvalid)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, err
	}
	if info, err := os.Lstat(Path(root)); err == nil && !info.Mode().IsRegular() {
		return Snapshot{}, fmt.Errorf("%w: configuration path must be a regular file", ErrInvalid)
	} else if err != nil {
		return Snapshot{}, err
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		return Snapshot{}, err
	}
	hash := sha256.Sum256(data)
	return Snapshot{Content: string(data), Revision: hex.EncodeToString(hash[:])}, nil
}

func Write(root string, data []byte, expected string) (Snapshot, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	return writeLocked(root, data, expected)
}

func Update(root, expected string, change func(Config) error) (Snapshot, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	current, err := Read(root)
	if err != nil {
		return Snapshot{}, err
	}
	if expected == "" || current.Revision != expected {
		return Snapshot{}, ErrConflict
	}
	if err := change(current.Config); err != nil {
		return Snapshot{}, err
	}
	data, err := Encode(current.Config)
	if err != nil {
		return Snapshot{}, err
	}
	return writeLocked(root, data, expected)
}

func Remove(root, expected string) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	current, err := ReadRaw(root)
	if err != nil {
		return err
	}
	if current.Revision != expected {
		return ErrConflict
	}
	return os.Remove(Path(root))
}

func writeLocked(root string, data []byte, expected string) (Snapshot, error) {
	if _, err := Parse(root, data); err != nil {
		return Snapshot{}, err
	}
	path := Path(root)
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, err
	}
	if expected != "" {
		hash := sha256.Sum256(current)
		if hex.EncodeToString(hash[:]) != expected || errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, ErrConflict
		}
	} else if err == nil {
		return Snapshot{}, ErrConflict
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Snapshot{}, err
	}
	if info, err := os.Lstat(filepath.Dir(path)); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Snapshot{}, fmt.Errorf("%w: configuration directory must be a real directory", ErrInvalid)
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return Snapshot{}, fmt.Errorf("%w: configuration path must be a regular file", ErrInvalid)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".workspace-*.yaml")
	if err != nil {
		return Snapshot{}, err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return Snapshot{}, err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return Snapshot{}, err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return Snapshot{}, err
	}
	if err := temp.Close(); err != nil {
		return Snapshot{}, err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return Snapshot{}, err
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return Read(root)
}

func Keys(config Config) []string {
	keys := make([]string, 0, len(config.Services))
	for key := range config.Services {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
