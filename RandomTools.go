package webtools

import (
	"fmt"
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
RemoveElementAtIndex removes element at specified index
*/
func RemoveElementAtIndex[T any](slice []T, index int) []T {
	if index < 0 || index >= len(slice) {
		return slice
	}
	if len(slice)-1 == index {
		return slice[:index]
	}
	if index == 0 {
		return slice[1:]
	}
	return append(slice[:index], slice[index+1:]...)
}

/*
InsertElementAtIndex inserts element at specified index
*/
func InsertElementAtIndex[T any](slice []T, index int, element T) []T {
	if index < 0 {
		return slice
	}
	if index > len(slice) {
		fmt.Println("Insert overflow")
		return append(slice, element)
	}
	if len(slice) == index {
		return append(slice, element)
	}
	if index == 0 {
		return append([]T{element}, slice...)
	}
	slice = append(slice, make([]T, 1)...)
	copy(slice[index+1:], slice[index:])
	slice[index] = element
	return slice
}

/*
IntAbs gets absolute value from any int value
*/
func IntAbs[T int | int8 | int16 | int32 | int64](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

/*
Server connection interface, removed because not used
*/
/*type IServerConn interface {
	Send([]byte)
	Close()
}*/
