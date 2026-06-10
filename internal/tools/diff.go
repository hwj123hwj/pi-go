package tools

import (
	"fmt"
	"strings"
)

const (
	maxDiffInputBytes = 256 * 1024
	maxDiffLines      = 2000
	maxDiffMatrix     = 1_000_000
)

func buildFileDiff(path, before, after string) string {
	if before == after {
		return ""
	}
	if len(before)+len(after) > maxDiffInputBytes {
		return fmt.Sprintf("--- %s\n+++ %s\n[diff omitted: input is %d bytes, limit is %d bytes]", path, path, len(before)+len(after), maxDiffInputBytes)
	}
	oldLines := splitDiffLines(before)
	newLines := splitDiffLines(after)
	if len(oldLines)+len(newLines) > maxDiffLines || len(oldLines)*len(newLines) > maxDiffMatrix {
		return fmt.Sprintf("--- %s\n+++ %s\n[diff omitted: %d old lines, %d new lines exceed diff budget]", path, path, len(oldLines), len(newLines))
	}
	lcs := make([][]int, len(oldLines)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- %s\n+++ %s\n", path, path))
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		if oldLines[i] == newLines[j] {
			b.WriteString(" ")
			b.WriteString(oldLines[i])
			b.WriteString("\n")
			i++
			j++
			continue
		}
		if lcs[i+1][j] >= lcs[i][j+1] {
			b.WriteString("-")
			b.WriteString(oldLines[i])
			b.WriteString("\n")
			i++
		} else {
			b.WriteString("+")
			b.WriteString(newLines[j])
			b.WriteString("\n")
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		b.WriteString("-")
		b.WriteString(oldLines[i])
		b.WriteString("\n")
	}
	for ; j < len(newLines); j++ {
		b.WriteString("+")
		b.WriteString(newLines[j])
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func splitDiffLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}
