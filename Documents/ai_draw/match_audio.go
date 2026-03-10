package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai_draw/pkg"
)

// WordData 表示 early_edu.json 中的单词数据结构
type WordData struct {
	ID           int    `json:"id"`
	Word         string `json:"word"`
	WordPinyin   string `json:"word_pinyin"`
	WordEnglish  string `json:"word_english"`
	Pic          string `json:"pic"`
	Audio        string `json:"audio"`
}

// EarlyEduData 表示 early_edu.json 的整体结构
type EarlyEduData map[string][]WordData

// UploadResult 上传结果
type UploadResult struct {
	ChineseName string
	AudioPath   string
	CDNPath     string
	URL         string
	Err         error
}

func main() {
	// 读取 early_edu.json 文件
	jsonPath := filepath.Join("ext", "early_edu.json")
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Printf("读取 JSON 文件失败: %v\n", err)
		os.Exit(1)
	}

	// 解析 JSON
	var data EarlyEduData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Printf("解析 JSON 失败: %v\n", err)
		os.Exit(1)
	}

	// 遍历双语单词音频目录
	audioDir := filepath.Join("ext", "双语单词音频")
	entries, err := os.ReadDir(audioDir)
	if err != nil {
		fmt.Printf("读取音频目录失败: %v\n", err)
		os.Exit(1)
	}

	var uploadResults []UploadResult
	var unmatchedFiles []struct {
		Filename    string
		ChineseName string
	}

	fmt.Println("=== 开始上传音频文件 ===")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		// 提取中文名称（文件名格式：中文名称 英文名称.mp3）
		chineseName := extractChineseName(filename)
		if chineseName == "" {
			continue
		}

		// 在所有分类中查找匹配的 word
		found := false
		for _, words := range data {
			for _, word := range words {
				if word.Word == chineseName {
					// 读取音频文件
					audioFilePath := filepath.Join(audioDir, filename)
					audioData, err := os.ReadFile(audioFilePath)
					if err != nil {
						uploadResults = append(uploadResults, UploadResult{
							ChineseName: chineseName,
							AudioPath:   word.Audio,
							Err:         fmt.Errorf("读取文件失败: %w", err),
						})
						found = true
						break
					}

					// 拼接 CDN 路径
					cdnPath := "audio/draw_board/early_edu/" + word.Audio

					// 上传到 WOS
					url, err := pkg.Upload2WOS(audioData, cdnPath)
					uploadResults = append(uploadResults, UploadResult{
						ChineseName: chineseName,
						AudioPath:   word.Audio,
						CDNPath:     cdnPath,
						URL:         url,
						Err:         err,
					})

					if err == nil {
						fmt.Printf("✓ %s -> %s\n", chineseName, url)
					} else {
						fmt.Printf("✗ %s 上传失败: %v\n", chineseName, err)
					}

					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			unmatchedFiles = append(unmatchedFiles, struct {
				Filename    string
				ChineseName string
			}{filename, chineseName})
		}
	}

	// 统计结果
	successCount := 0
	failCount := 0
	for _, result := range uploadResults {
		if result.Err == nil {
			successCount++
		} else {
			failCount++
		}
	}

	fmt.Printf("\n=== 上传结果统计 ===\n")
	fmt.Printf("上传成功: %d 个\n", successCount)
	fmt.Printf("上传失败: %d 个\n", failCount)
	fmt.Printf("未匹配: %d 个\n", len(unmatchedFiles))

	// 打印失败的记录
	if failCount > 0 {
		fmt.Printf("\n=== 上传失败的记录 ===\n")
		for _, result := range uploadResults {
			if result.Err != nil {
				fmt.Printf("%s (%s): %v\n", result.ChineseName, result.AudioPath, result.Err)
			}
		}
	}

	// 打印未匹配的记录
	if len(unmatchedFiles) > 0 {
		fmt.Printf("\n=== 未匹配记录 (%d 个) ===\n", len(unmatchedFiles))
		for _, uf := range unmatchedFiles {
			fmt.Printf("%s (中文: %s)\n", uf.Filename, uf.ChineseName)
		}
	}

	// 保存成功上传的 URL（保存相对路径）
	if successCount > 0 {
		outputFile, _ := os.Create("uploaded_urls.txt")
		defer outputFile.Close()

		for _, result := range uploadResults {
			if result.Err == nil && result.CDNPath != "" {
				outputFile.WriteString(result.CDNPath + "\n")
			}
		}
		fmt.Printf("\n✓ 成功上传的 URL 已保存到 uploaded_urls.txt\n")
	}
}

// extractChineseName 从文件名中提取中文名称
// 文件名格式：中文名称 英文名称.mp3
func extractChineseName(filename string) string {
	// 去掉扩展名
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 按空格分割，取第一部分（中文名称）
	parts := strings.SplitN(name, " ", 2)
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}
