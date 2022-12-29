package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// run test command
// go test -v          								 	for all test
// go test -v -run=TestHelloWorld 			for individual func
// go test ./...												for all test in parent folder
func TestMain(m *testing.M) {
	//before
	fmt.Println("\nSTART UNIT TEST 'helper.go'")

	m.Run()

	//after
	fmt.Println("END UNIT TEST 'helper.go'")
}

func Test_StringInSlice(t *testing.T) {
	asserts := assert.New(t)
	keys := []string{"a", "b", "c", "d", "e", "f", "g"}

	asserts.True(StringInSlice("a", keys))
	asserts.True(StringInSlice("b", keys))
	asserts.True(StringInSlice("c", keys))
	asserts.True(StringInSlice("d", keys))
	asserts.True(StringInSlice("e", keys))
	asserts.True(StringInSlice("f", keys))
	asserts.True(StringInSlice("g", keys))
	asserts.False(StringInSlice("gg", keys))
}

// func Test_ToDoc(t *testing.T) {
// 	asserts := assert.New(t)
// 	var data *user.UpdateUser = &entity.UpdateUser{
// 		Id:        primitive.NewObjectID(),
// 		Title:     "Title",
// 		Content:   "Content",
// 		Image:     "Image",
// 		User:      "User",
// 		CreateAt:  time.Now(),
// 		UpdatedAt: time.Now(),
// 	}
// 	doc, err := ToDoc(data)

// 	asserts.Nil(err, "error should nil")
// 	asserts.NotNil(doc, "doc should not nil")

// 	keys := []string{"_id", "title", "content", "image", "user", "created_at", "updated_at"}
// 	if err == nil {
// 		for _, val := range *doc {
// 			asserts.True(StringInSlice(val.Key, keys), val.Key+" unavailable")
// 		}
// 	}
// }

func Test_InputPassword(t *testing.T) {
	asserts := assert.New(t)

	valid, err := IsValidPassword("password")
	asserts.True(valid)

	valid, err = IsValidPassword("123456")
	asserts.True(valid)

	valid, err = IsValidPassword("pass")
	asserts.True(!valid)
	asserts.Equal(err.Error(), "password to short")

	valid, err = IsValidPassword("")
	asserts.True(!valid)
	asserts.Equal(err.Error(), "password to can not empty")

}

func Test_InputName(t *testing.T) {
	asserts := assert.New(t)
	valid, err := IsValidName("Royyan Wibisono")
	asserts.True(valid)

	valid, err = IsValidName("!@@#$fsdl&*()(_)&&&")
	asserts.True(valid)

	valid, err = IsValidName("")
	asserts.True(!valid)
	asserts.Equal(err.Error(), "name can not empty")

	valid, err = IsValidName("01234567890123456789012345678901234567890123456789a")
	asserts.True(!valid)
	asserts.Equal(err.Error(), "name to long, max 50 characters")
}

func Test_InputUsername(t *testing.T) {
	asserts := assert.New(t)

	valid, err := IsValidUsername("ujang")
	asserts.True(valid)

	valid, err = IsValidUsername("ujang-geboy")
	asserts.True(valid)

	valid, err = IsValidUsername("ujang_sembur")
	asserts.True(valid)

	valid, err = IsValidUsername("me")
	asserts.True(valid)

	valid, err = IsValidUsername("12345678901234567890")
	asserts.True(valid)

	valid, err = IsValidUsername("")
	asserts.True(!valid)
	asserts.Equal(err.Error(), "username to can not empty")

	valid, err = IsValidUsername("a")
	asserts.True(!valid)
	asserts.Equal(err.Error(), "username to short")

	valid, err = IsValidUsername("123456789012345678901")
	asserts.True(!valid)
	asserts.Equal(err.Error(), "username to long, max 20 characters")

	valid, err = IsValidUsername("__me")
	asserts.True(!valid)

	valid, err = IsValidUsername("-desk")
	asserts.True(!valid)

	valid, err = IsValidUsername("ujang delman")
	asserts.True(!valid)
}

func Test_InputEmail(t *testing.T) {
	asserts := assert.New(t)

	valid := IsValidEmail("user@mail.com")
	asserts.True(valid)

	valid = IsValidEmail("user123@mail.com")
	asserts.True(valid)

	valid = IsValidEmail("user-123@mail.com")
	asserts.True(valid)

	valid = IsValidEmail("O`Raily@my-mail.com")
	asserts.True(valid)

	valid = IsValidEmail("qwerty")
	asserts.True(!valid)

	valid = IsValidEmail("user123@mail")
	asserts.True(!valid)

}
func Test_UUIDvalidate(t *testing.T) {
	asserts := assert.New(t)
	valid := IsValidUid("267f591c-3de1-4dec-819a-00fe801de8ed")
	asserts.True(valid)

	valid = IsValidUid("")
	asserts.True(!valid)
}

func Test_CopyStruct(t *testing.T) {
	asserts := assert.New(t)

	type Man struct {
		Name string
		Age  int
	}

	type Person struct {
		FirstName string
		LastName  string
		Age       int
	}

	// Create a usr struct with some values
	usr := Man{Name: "John", Age: 30}
	// Create a new person struct with zero values
	var person Person
	// Copy the values from user to person
	CopyStruct(&usr, &person)

	asserts.Equal(person.Age, usr.Age)

	type ResponseUser struct {
		UID          string `json:"uid"`
		Name         string `json:"name"`
		Username     string `json:"username"`
		Email        string `json:"email"`
		JWT          string `json:"jwt"`
		IsRegistered bool   `json:"isregistered"`
	}

	type DBUser struct {
		UID      string `json:"uid" bson:"uid"`
		Name     string `json:"name" bson:"name"`
		Username string `json:"username" bson:"username"`
		Password string `json:"password" bson:"password"`
		Email    string `json:"email" bson:"email"`
	}

	test := &DBUser{
		UID:      "123-456-999",
		Name:     "ojan",
		Username: "ojan",
		Password: "fdsah-kfiuynowef-yoifmiwf",
		Email:    "ojan@mail.com",
	}

	var res ResponseUser

	CopyStruct(test, &res)

	asserts.Equal(test.Name, res.Name)
	asserts.Equal(test.Email, res.Email)
	asserts.Equal("123-456-999", res.UID)

}
