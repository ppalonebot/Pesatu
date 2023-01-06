package auth

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	//before
	fmt.Println("\nSTART UNIT TEST 'mail.go'")

	m.Run()

	//after
	fmt.Println("END UNIT TEST 'mail.go'")
}

func Test_GetAbsolutePath(t *testing.T) {
	asserts := assert.New(t)
	exePath, _ := os.Executable()
	asserts.Contains(exePath, "#")

}
func Test_ReadHtml(t *testing.T) {
	asserts := assert.New(t)
	// Read the HTML template file into a variable
	var body bytes.Buffer
	templateData, err := template.ParseFiles("mailcode_template.html")
	asserts.Nil(err)
	err = templateData.Execute(&body, struct{ Code string }{Code: "676767"})
	asserts.Nil(err)
	asserts.Contains(body.String(), "676767")
}

// func Test_SendEmail(t *testing.T) {
// 	asserts := assert.New(t)
// 	to := []string{"dolbytoby@gmail.com"}
// 	cc := []string{"royyanwibisono@gmail.com"}
// 	subject := "Test mail"
// 	message := "Hello"

// 	err := SendMail(to, cc, subject, message)
// 	asserts.Nil(err)
// 	//err := SendGoEmail("dolbytoby@gmail.com", "", "Hello, World!", "Hello, World!")
// 	//asserts.Nil(err)
// }

// func Test_SendCodeEmail(t *testing.T) {
// 	asserts := assert.New(t)
// 	err := SendCodeMail("dolbytoby@gmail.com", "123456")
// 	asserts.Nil(err)
// 	//err := SendGoEmail("dolbytoby@gmail.com", "", "Hello, World!", "Hello, World!")
// 	//asserts.Nil(err)
// }
