package webtools

import (
	"math/rand/v2"
	"strconv"
	"time"
)

const ALPHABET_LETTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const NUMBER_LETTERS = "0123456789"

/*
GenerateRandomID generates random Id based on random and current time
*/
func GenerateRandomID() string {
	return strconv.FormatUint(rand.Uint64(), 36) + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

/*
GenerateRandomString generates random string
*/
func GenerateRandomString(lenght int) string {
	letters := NUMBER_LETTERS + ALPHABET_LETTERS
	result := ""
	for i := 0; i < lenght; i++ {
		result += string(letters[rand.IntN(len(letters))])
	}
	return result
}

/*
RemoveElement removes element from slice
*/
func RemoveElement[T comparable](array []T, item T) []T {
	result := make([]T, 0)
	for i := 0; i < len(array); i++ {
		if array[i] != item {
			result = append(result, array[i])
		}
	}
	return result
}

/*
Server connection interface, removed because not used
*/
/*type IServerConn interface {
	Send([]byte)
	Close()
}*/
