package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/phpdave11/gofpdf"
)

type CV struct {
	Language       string `json:"language"`
	Name           string `json:"name"`
	Title          string `json:"title"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Location       string `json:"location"`
	LinkedIn       string `json:"linkedin"`
	Website        string `json:"website"`
	Summary        string `json:"summary"`
	Education      string `json:"education"`
	Experience     string `json:"experience"`
	Projects       string `json:"projects"`
	Organizations  string `json:"organizations"`
	Skills         string `json:"skills"`
	Certifications string `json:"certifications"`
	Languages      string `json:"languages"`
	Photo          string `json:"photo"`
}

type section struct {
	label string
	value string
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		http.ServeFile(w, r, "templates/index.html")
	})

	mux.HandleFunc("/translate", translateCV)
	mux.HandleFunc("/download/docx", downloadDOCX)
	mux.HandleFunc("/download/pdf", downloadPDF)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Println("CV ATS Builder: http://localhost:8080")
	log.Fatal(server.ListenAndServe())
}

func decodeCV(r *http.Request) (CV, error) {
	var cv CV

	r.Body = http.MaxBytesReader(nil, r.Body, 5<<20)

	if err := json.NewDecoder(r.Body).Decode(&cv); err != nil {
		return cv, fmt.Errorf("Data tidak valid atau terlalu besar")
	}

	cv.Name = strings.TrimSpace(cv.Name)
	if cv.Name == "" {
		return cv, fmt.Errorf("Nama lengkap wajib diisi")
	}

	if cv.Language != "en" {
		cv.Language = "id"
	}

	return cv, nil
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func translateCV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cv, err := decodeCV(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	source, target := "id", "en"
	if cv.Language == "en" {
		source, target = "en", "id"
	}

	fields := []*string{
		&cv.Title,
		&cv.Summary,
		&cv.Education,
		&cv.Experience,
		&cv.Projects,
		&cv.Organizations,
		&cv.Skills,
		&cv.Certifications,
		&cv.Languages,
	}

	for _, field := range fields {
		if strings.TrimSpace(*field) == "" {
			continue
		}

		translated, err := translateText(r.Context(), *field, source, target)
		if err != nil {
			respondError(w, http.StatusBadGateway, err.Error())
			return
		}
		*field = translated
	}

	if target == "en" {
		cv.Language = "en"
	} else {
		cv.Language = "id"
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(cv)
}

func translateText(parentCtx context.Context, text, source, target string) (string, error) {
	if source != "id" && source != "en" {
		return "", fmt.Errorf("Bahasa sumber tidak didukung")
	}
	if target != "id" && target != "en" {
		return "", fmt.Errorf("Bahasa tujuan tidak didukung")
	}
	if source == target {
		return text, nil
	}

	endpoint := "https://api.mymemory.translated.net/get"

	ctx, cancel := context.WithTimeout(parentCtx, 60*time.Second)
	defer cancel()

	// Pecah teks panjang menjadi bagian yang lebih kecil.
	words := strings.Fields(text)
	if len(words) == 0 {
		return text, nil
	}

	var result strings.Builder
	var chunk []string

	translateChunk := func(part string) (string, error) {
		params := url.Values{}
		params.Set("q", part)
		params.Set("langpair", source+"|"+target)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			endpoint+"?"+params.Encode(),
			nil,
		)
		if err != nil {
			return "", err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("Tidak dapat mengakses layanan terjemahan: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("Layanan terjemahan gagal (HTTP %d)", resp.StatusCode)
		}

		var data struct {
			ResponseStatus  int    `json:"responseStatus"`
			ResponseDetails string `json:"responseDetails"`
			ResponseData    struct {
				TranslatedText string `json:"translatedText"`
			} `json:"responseData"`
		}

		if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&data); err != nil {
			return "", fmt.Errorf("Respons terjemahan tidak valid")
		}

		if data.ResponseStatus != 200 || data.ResponseData.TranslatedText == "" {
			if data.ResponseDetails != "" {
				return "", fmt.Errorf("Terjemahan gagal: %s", data.ResponseDetails)
			}
			return "", fmt.Errorf("Layanan tidak mengembalikan hasil terjemahan")
		}

		return data.ResponseData.TranslatedText, nil
	}

	for _, word := range words {
		candidate := strings.Join(append(chunk, word), " ")

		if len(candidate) > 400 && len(chunk) > 0 {
			translated, err := translateChunk(strings.Join(chunk, " "))
			if err != nil {
				return "", err
			}

			if result.Len() > 0 {
				result.WriteString(" ")
			}
			result.WriteString(translated)
			chunk = []string{word}
		} else {
			chunk = append(chunk, word)
		}
	}

	if len(chunk) > 0 {
		translated, err := translateChunk(strings.Join(chunk, " "))
		if err != nil {
			return "", err
		}
		if result.Len() > 0 {
			result.WriteString(" ")
		}
		result.WriteString(translated)
	}

	return result.String(), nil
}

func downloadPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cv, err := decodeCV(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")

	pdf.AddUTF8Font("Arial", "", "C:/Windows/Fonts/arial.ttf")
	pdf.AddUTF8Font("Arial", "B", "C:/Windows/Fonts/arialbd.ttf")

	pdf.SetMargins(18, 16, 18)
	pdf.SetAutoPageBreak(true, 16)
	pdf.AddPage()

	if photo, format, err := decodePhoto(cv.Photo); err == nil && len(photo) > 0 {
		name := "profile"
		pdf.RegisterImageReader(name, format, bytes.NewReader(photo))
		pdf.ImageOptions(name, 165, 16, 27, 27, false, gofpdf.ImageOptions{ImageType: format, ReadDpi: true}, 0, "")
	}

	pdf.SetFont("Arial", "B", 18)
	pdf.MultiCell(140, 8, cv.Name, "", "L", false)

	if cv.Title != "" {
		pdf.SetFont("Arial", "", 11)
		pdf.MultiCell(140, 6, cv.Title, "", "L", false)
	}

	contact := strings.Join(nonempty([]string{
		cv.Email,
		cv.Phone,
		cv.Location,
		cv.LinkedIn,
		cv.Website,
	}), " | ")

	if contact != "" {
		pdf.SetFont("Arial", "", 9)
		pdf.MultiCell(0, 5, contact, "", "L", false)
	}

	for _, s := range sectionLabels(cv) {
		if strings.TrimSpace(s.value) == "" {
			continue
		}

		pdf.Ln(3)
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(0, 6, s.label, "", 1, "L", false, 0, "")
		y := pdf.GetY()
		pdf.Line(18, y, 192, y)
		pdf.Ln(2)

		pdf.SetFont("Arial", "", 10)
		for _, line := range strings.Split(s.value, "\n") {
			pdf.MultiCell(0, 5, line, "", "L", false)
		}
	}

	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		http.Error(w, "Gagal membuat PDF: "+err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="CV-ATS.pdf"`)
	_, _ = w.Write(output.Bytes())
}

func decodePhoto(data string) ([]byte, string, error) {
	if data == "" {
		return nil, "", nil
	}

	parts := strings.SplitN(data, ",", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("Format foto tidak valid")
	}

	format := ""
	switch {
	case strings.Contains(parts[0], "image/png"):
		format = "PNG"
	case strings.Contains(parts[0], "image/jpeg"):
		format = "JPG"
	default:
		return nil, "", fmt.Errorf("Foto harus berupa JPG atau PNG")
	}

	raw, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", err
	}

	_, actualFormat, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("File foto tidak valid")
	}
	if actualFormat != "jpeg" && actualFormat != "png" {
		return nil, "", fmt.Errorf("Format foto tidak didukung")
	}

	return raw, format, nil
}

func nonempty(items []string) []string {
	out := []string{}
	for _, v := range items {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func sectionLabels(c CV) []section {
	if c.Language == "en" {
		return []section{
			{"PROFESSIONAL SUMMARY", c.Summary},
			{"EDUCATION", c.Education},
			{"EXPERIENCE", c.Experience},
			{"PROJECTS", c.Projects},
			{"ORGANIZATIONS", c.Organizations},
			{"SKILLS", c.Skills},
			{"CERTIFICATIONS & AWARDS", c.Certifications},
			{"LANGUAGES", c.Languages},
		}
	}

	return []section{
		{"RINGKASAN PROFESIONAL", c.Summary},
		{"PENDIDIKAN", c.Education},
		{"PENGALAMAN", c.Experience},
		{"PROYEK", c.Projects},
		{"ORGANISASI", c.Organizations},
		{"KEAHLIAN", c.Skills},
		{"SERTIFIKASI & PENGHARGAAN", c.Certifications},
		{"BAHASA", c.Languages},
	}
}

func downloadDOCX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cv, err := decodeCV(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, err := makeDOCX(cv)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Gagal membuat dokumen Word: "+err.Error())
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	)
	w.Header().Set("Content-Disposition", `attachment; filename="CV-ATS.docx"`)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	_, _ = w.Write(data)
}
