package user

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	//before
	fmt.Println("\nSTART UNIT TEST 'user'")

	m.Run()

	//after
	fmt.Println("END UNIT TEST 'user'")
}

func Test_Timedelta(t *testing.T) {
	asserts := assert.New(t)
	delta := time.Duration.Seconds(5 * time.Second)
	asserts.Equal("5.0", fmt.Sprintf("%.1f", delta))
}
