package webtools

import (
	"math/rand/v2"
	"strconv"
	"time"
)

/*
Generates random Id based on random and current time
*/
func GenerateRandomId() string {
	return strconv.FormatUint(rand.Uint64(), 36) + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

/*
Generates random string
*/
func GenerateRandomString(lenght int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := ""
	for i := 0; i < lenght; i++ {
		result += string(letters[rand.IntN(len(letters))])
	}
	return result
}
