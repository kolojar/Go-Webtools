package webtools

import (
	"math/rand/v2"
	"strconv"
	"time"
)

/*
AlphabetLetters are all English alphabet letters
*/
const AlphabetLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

/*
NumberLetters are all number characters
*/
const NumberLetters = "0123456789"

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
	letters := NumberLetters + AlphabetLetters
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
CeilDivision divides number a by b and threats it as ceiled value
*/
func CeilDivision[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64](a, b T) T {
	return (a + b - 1) / b
}

/*
Server connection interface, removed because not used
*/
/*type IServerConn interface {
	Send([]byte)
	Close()
}*/
