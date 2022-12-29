package auth

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/gomail.v2"
)

const CONFIG_SMTP_HOST = "smtp.gmail.com"
const CONFIG_SMTP_PORT = 587
const CONFIG_SENDER_NAME = "PESATU DEVELOPER <developerroyyan@gmail.com>"
const CONFIG_AUTH_EMAIL = "developerroyyan@gmail.com"

// vist Google account
// enable 2-step Verification
// follow instruction
// go back
// click "App password"
// sign in
// on Select the app and device you want to generate the app password for.
// choose cutom
// input appname then Generate
// copy password bellow, and done
const CONFIG_AUTH_PASSWORD = "hnpsifhwswjntvsd"

func SendMail(to []string, cc []string, subject, message string) error {
	body := "From: " + CONFIG_SENDER_NAME + "\n" +
		"To: " + strings.Join(to, ",") + "\n" +
		"Cc: " + strings.Join(cc, ",") + "\n" +
		"Subject: " + subject + "\n\n" +
		message

	auth := smtp.PlainAuth("", CONFIG_AUTH_EMAIL, CONFIG_AUTH_PASSWORD, CONFIG_SMTP_HOST)
	smtpAddr := fmt.Sprintf("%s:%d", CONFIG_SMTP_HOST, CONFIG_SMTP_PORT)

	err := smtp.SendMail(smtpAddr, auth, CONFIG_AUTH_EMAIL, append(to, cc...), []byte(body))
	if err != nil {
		return err
	}

	return nil
}

func SendHtmlMail(to string, subject string, data any, template_thtml string) error {
	// Read the HTML template file into a variable
	var body bytes.Buffer
	templateData, err := template.ParseFiles(fmt.Sprintf("../template/%s", template_thtml))
	if err != nil {
		// Get the current working directory
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		// Get the absolute path of the current file
		absPath, err := filepath.Abs(wd)
		if err != nil {
			return err
		}

		templateData, err = template.ParseFiles(fmt.Sprintf("%s/template/%s", absPath, template_thtml))
		if err != nil {
			return err
		}
	}

	err = templateData.Execute(&body, data)
	if err != nil {
		return err
	}

	// Create a new gomail message
	msg := gomail.NewMessage()
	// Set the subject, recipient, and sender of the message
	msg.SetHeader("From", CONFIG_SENDER_NAME)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)
	// Set the HTML body of the message
	msg.SetBody("text/html", body.String())

	// Set up the email server and send the message
	m := gomail.NewDialer(CONFIG_SMTP_HOST, CONFIG_SMTP_PORT, CONFIG_AUTH_EMAIL, CONFIG_AUTH_PASSWORD)
	if err := m.DialAndSend(msg); err != nil {
		return err
	}

	return nil
}

func SendCodeMail(to string, data any) error {
	return SendHtmlMail(to, "Registration Code", data, "mailcode_t.html")
}

func SendForgotPwdMail(to string, data any) error {
	return SendHtmlMail(to, "Reset Password", data, "mailforgotpwd_t.html")
}
