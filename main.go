package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"time"
)

// Struct to parse frontend form data
type ContactForm struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Company   string `json:"company"`
	Message   string `json:"message"`
	Token     string `json:"recaptchaToken"`
}

// Recaptcha verification response
type RecaptchaResponse struct {
	Success bool    `json:"success"`
	Score   float64 `json:"score"`
}

// Email sending handler
func contactHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var form ContactForm
	err := json.NewDecoder(r.Body).Decode(&form)
	if err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// === RECAPTCHA VALIDATION ===
	if !verifyRecaptcha(form.Token) {
		http.Error(w, "reCAPTCHA failed", http.StatusUnauthorized)
		return
	}

	// === BASIC VALIDATIONS ===
	if !isValidEmail(form.Email) {
		http.Error(w, "Invalid email", http.StatusBadRequest)
		return
	}
	if form.FirstName == "" || form.LastName == "" || form.Message == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// === EMAIL COMPOSITION ===
	from := os.Getenv("SMTP_EMAIL")
	password := os.Getenv("SMTP_PASSWORD")
	to := "info@next-kiosk.com"

	subject := "New Contact Form Submission"
	body := fmt.Sprintf(`
	New message from: %s %s
	Email: %s
	Phone: %s
	Company: %s

	Message:
	%s
	`, form.FirstName, form.LastName, form.Email, form.Phone, form.Company, form.Message)

	msg := []byte("To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body)

	auth := smtp.PlainAuth("", from, password, "smtpout.secureserver.net")

	err = smtp.SendMail("smtpout.secureserver.net:587", auth, from, []string{to}, msg)
	if err != nil {
		log.Printf("Email send error: %v", err)
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}

	// SUCCESS RESPONSE
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Validate reCAPTCHA v3 token
func verifyRecaptcha(token string) bool {
	secret := os.Getenv("RECAPTCHA_SECRET")
	if secret == "" {
		log.Println("Missing RECAPTCHA_SECRET")
		return false
	}

	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify",
		map[string][]string{
			"secret":   {secret},
			"response": {token},
		},
	)
	if err != nil {
		log.Println("reCAPTCHA HTTP error:", err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result RecaptchaResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Println("reCAPTCHA parse error:", err)
		return false
	}

	log.Println("reCAPTCHA score:", result.Score)
	return result.Success && result.Score > 0.5
}

func main() {
	// sending test mail to verify SMTP settings
	if err := sendTestMail(); err != nil {
		log.Println("Test mail failed:", err)
	} else {
		log.Println("Test mail sent successfully")
	}

	http.Handle("/api/contact", corsMiddleware(http.HandlerFunc(contactHandler)))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from your frontend domain
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:3000" ||
			origin == "https://next-kiosk.com" ||
			origin == "https://next-kiosk.netlify.app" ||
			origin == "http://next-kiosk.netlify.app" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func sendTestMail() error {
	from := os.Getenv("SMTP_EMAIL")
	password := os.Getenv("SMTP_PASSWORD")
	to := "nextkiosksolutions@gmail.com"

	subject := "✅ Mail System Check - Next Kiosk"
	body := fmt.Sprintf("Mail functionality has been deployed and it's working. Time: %s", time.Now().Format("2006-01-02 15:04:05"))

	msg := []byte(
		"From: Next Kiosk <" + from + ">\r\n" +
			"To: Muhammet Aydın <" + to + ">\r\n" +
			"Subject: " + subject + "\r\n" +
			"Date: " + formatDateRFC5322() + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"Content-Transfer-Encoding: 7bit\r\n" +
			"\r\n" + body)

	auth := smtp.PlainAuth("", from, password, "smtpout.secureserver.net")

	err := smtp.SendMail("smtpout.secureserver.net:587", auth, from, []string{to}, msg)
	if err != nil {
		log.Printf("smtp.SendMail failed: %v", err)
		return fmt.Errorf("failed to send test mail: %w", err)
	}
	log.Println("✅ Test mail sent successfully to", to)
	return nil
}

func formatDateRFC5322() string {
	return time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700")
}

func isValidEmail(email string) bool {
	reg := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	return reg.MatchString(email)
}
