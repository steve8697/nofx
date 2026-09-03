package decision

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	skillMu    sync.RWMutex
	skillFiles = map[string]string{}
	skillsDir  = filepath.Join("prompts", "skills")
)

// LoadPlaybooks 从 prompts/skills/*.md 载入按需注入的交易 playbook。
func LoadPlaybooks() error {
	skillMu.Lock()
	defer skillMu.Unlock()
	skillFiles = map[string]string{}

	files, err := filepath.Glob(filepath.Join(skillsDir, "*.md"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		log.Printf("⚠️  未找到 %s/*.md，将回退到完整 adaptive 模板", skillsDir)
		return nil
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("⚠️  读取 skill 失败 %s: %v", file, err)
			continue
		}
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		skillFiles[name] = string(content)
		log.Printf("  📎 加载交易 skill: %s", name)
	}
	return nil
}

func playbookContent(name string) (string, bool) {
	skillMu.RLock()
	defer skillMu.RUnlock()
	c, ok := skillFiles[name]
	return c, ok
}

// SelectPlaybooks 按持仓状态选择要注入的 skill。规则文本不改，只少灌无关章节。
func SelectPlaybooks(positionCount, maxPositions int) []string {
	if maxPositions <= 0 {
		maxPositions = 3
	}
	var names []string
	canOpen := positionCount < maxPositions
	if positionCount > 0 {
		names = append(names, "manage")
	}
	if canOpen {
		names = append(names, "entry", "filters", "sizing")
	}
	names = append(names, "context")
	return names
}

func assemblePlaybooks(names []string) string {
	var sb strings.Builder
	for _, name := range names {
		content, ok := playbookContent(name)
		if !ok || strings.TrimSpace(content) == "" {
			continue
		}
		sb.WriteString("\n\n---\n")
		sb.WriteString(fmt.Sprintf("# Injected skill: %s\n\n", name))
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	return sb.String()
}

func playbooksLoaded() bool {
	skillMu.RLock()
	defer skillMu.RUnlock()
	return len(skillFiles) > 0
}
