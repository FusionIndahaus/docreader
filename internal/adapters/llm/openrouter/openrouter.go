package openrouter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Client struct {
	APIKey    string
	BaseURL   string
	Model     string
	SiteURL   string
	SiteTitle string
}

func (c Client) Process(ctx context.Context, message string, fileName string, contentType string, fileBytes []byte) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", fmt.Errorf("не задан OPENROUTER_API_KEY")
	}
	type imageURL struct {
		Url string `json:"url"`
	}
	type contentPart struct {
		Type     string    `json:"type"`
		Text     string    `json:"text,omitempty"`
		ImageURL *imageURL `json:"image_url,omitempty"`
	}
	type chatMessage struct {
		Role    string        `json:"role"`
		Content []contentPart `json:"content"`
	}
	type chatRequest struct {
		Model    string        `json:"model"`
		Messages []chatMessage `json:"messages"`
	}
	type choiceMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type choice struct {
		Index   int           `json:"index"`
		Message choiceMessage `json:"message"`
	}
	type chatResponse struct {
		Choices []choice `json:"choices"`
	}

	systemContent := `You are an assistant for extracting specific data from documents. 
Return ONLY the exact values explicitly requested by the user. 
For each requested value, output ONLY ONE line with the most suitable result. 
Do not output all similar values you see - choose only the single most appropriate one for each category which user want to exctract. 
The output must contain raw values separated by new lines, 
without quotes, equals signs, labels, explanations, or extra commentary. 
If a requested value is missing, output the word MISSING on its own line. 
Never invent or add information beyond what the user asked for.`

	userText := fmt.Sprintf(
		"File: %s\nUser request: %s",
		truncate(fileName, 120),
		truncate(message, 4000),
	)
	userParts := []contentPart{
		{Type: "text", Text: userText},
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(ct, "image/") {
		b64 := base64.StdEncoding.EncodeToString(fileBytes)
		dataURI := "data:" + ct + ";base64," + b64
		userParts = append(userParts, contentPart{Type: "image_url", ImageURL: &imageURL{Url: dataURI}})
	} else if strings.Contains(ct, "pdf") {
		if pngB64, err := rasterizePDFFirstPageToPNGBase64(fileBytes); err == nil && strings.TrimSpace(pngB64) != "" {
			dataURI := "data:image/png;base64," + pngB64
			userParts = append(userParts, contentPart{Type: "image_url", ImageURL: &imageURL{Url: dataURI}})
		}
	}

	reqBody := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: []contentPart{{Type: "text", Text: systemContent}}},
			{Role: "user", Content: userParts},
		},
	}

	buf, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if c.SiteURL != "" {
		req.Header.Set("HTTP-Referer", c.SiteURL)
	}
	if c.SiteTitle != "" {
		req.Header.Set("X-Title", c.SiteTitle)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("модель вернула ошибку %d: %s", resp.StatusCode, truncate(string(body), 800))
	}
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", nil
	}
	return strings.TrimSpace(cr.Choices[0].Message.Content), nil
}

func rasterizePDFFirstPageToPNGBase64(pdfBytes []byte) (string, error) {
	f, err := os.CreateTemp("", "docai_input_*.pdf")
	if err != nil {
		return "", err
	}
	pdfPath := f.Name()
	_, werr := f.Write(pdfBytes)
	cerr := f.Close()
	if werr != nil {
		os.Remove(pdfPath)
		return "", werr
	}
	if cerr != nil {
		os.Remove(pdfPath)
		return "", cerr
	}
	defer os.Remove(pdfPath)
	outPrefix := strings.TrimSuffix(pdfPath, ".pdf")
	cmd := exec.Command("pdftoppm", "-png", "-f", "1", "-l", "1", "-singlefile", pdfPath, outPrefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = out
		return "", fmt.Errorf("pdftoppm error: %w", err)
	}
	pngPath := outPrefix + ".png"
	defer os.Remove(pngPath)
	data, err := os.ReadFile(pngPath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
