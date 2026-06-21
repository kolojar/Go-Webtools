package filesystem

import (
	"fmt"
	"slices"

	webtools "github.com/kolojar/Go-Webtools"
)

//func ChangesInSequence(old []byte, new []byte) []webtools.KeyValuePair[int, FileSystemEventType] {
//	result := make([]webtools.KeyValuePair[int, FileSystemEventType], 0)
//	var pos int = 0
//	for pos = 0; pos < len(new); pos++ {
//		if len(old) <= pos {
//			//All after is added
//			for _ = pos; pos < len(new); pos++ {
//				result = append(result, webtools.KeyValuePair[int, FileSystemEventType]{Key: pos, Value: FSEventCreated})
//			}
//			break
//		}
//		if old[pos] != new[pos] {
//			result = append(result, webtools.KeyValuePair[int, FileSystemEventType]{Key: pos, Value: FSEventModified})
//		}
//	}
//	if len(old) > pos {
//		//All after is removed
//		for _ = pos; pos < len(old); pos++ {
//			result = append(result, webtools.KeyValuePair[int, FileSystemEventType]{Key: pos, Value: FSEventDeleted})
//		}
//	}
//	return result
//}

type DifferenceEntry[T comparable] struct {
	Position    int
	Character   T
	IsInsertion bool
}

func appendAtTop[T comparable](slice []DifferenceEntry[T], item DifferenceEntry[T]) []DifferenceEntry[T] {
	//Inserts value from the start - biggest position at the top
	for i := 0; i < len(slice); i++ {
		if slice[i].Position < item.Position {
			return webtools.InsertElementAtIndex(slice, i, item)
		}
	}
	return append(slice, item)
}

func appendAtBottom[T comparable](slice []DifferenceEntry[T], item DifferenceEntry[T]) []DifferenceEntry[T] {
	//Insert value from the end - biggest position at the bottom
	for i := len(slice) - 1; i >= 0; i-- {
		if slice[i].Position < item.Position {
			return webtools.InsertElementAtIndex(slice, i+1, item)
		}
	}
	return webtools.InsertElementAtIndex(slice, 0, item)
}

type differenceStaticLetter struct {
	Letter rune
	Index  int
}

func appendAtBottomDiffrerenceLetter(slice []differenceStaticLetter, item differenceStaticLetter) []differenceStaticLetter {
	//Insert value from the end - biggest position at the bottom
	for i := len(slice) - 1; i >= 0; i-- {
		if slice[i].Index < item.Index {
			return webtools.InsertElementAtIndex(slice, i+1, item)
		}
	}
	return webtools.InsertElementAtIndex(slice, 0, item)
}

/*
DiffTrim is trimming function for diff algorithms, returns trimmed arrays and offset from beginning (for valid DifferenceEntry)
*/
func DiffTrim[T comparable](old []T, new []T) ([]T, []T, int) {
	//Find prefix
	var prefixLen = 0
	for prefixLen < len(old) && prefixLen < len(new) && old[prefixLen] == new[prefixLen] {
		prefixLen++
	}

	//Find suffix
	var suffixLen = 0
	for suffixLen < len(old)-prefixLen && suffixLen < len(new)-prefixLen && old[len(old)-1-suffixLen] == new[len(new)-1-suffixLen] {
		suffixLen++
	}
	return old[prefixLen : len(old)-suffixLen], new[prefixLen : len(new)-suffixLen], prefixLen
}

/*
DiffInStringLCS checks for differences in two arrays. Returns array of changes
It is recommended to use DiffInStringLCSAlt, because it is more effective
Simplified LCS diff check - https://en.wikipedia.org/wiki/Longest_common_subsequence
*/
func DiffInStringLCS[T comparable](old []T, new []T) []DifferenceEntry[T] {
	//Check if not empty
	old, new, offset := DiffTrim(old, new)
	if len(old) == 0 && len(new) == 0 {
		return nil
	}
	//REMOVED SAME OLD AND NEW - LOTS OF TIMES BOTH HAVE SAME
	//sameOld := make([]differenceStaticLetter, 0)
	//lookupOldRunes := make(map[rune][]int, 0)
	//for index, char := range oldRunes {
	//	if lookupOldRunes[char] == nil {
	//		lookupOldRunes[char] = make([]int, 0)
	//	}
	//	lookupOldRunes[char] = append(lookupOldRunes[char], index)
	//}

	////Get same letters
	//sameNew := make([]differenceStaticLetter, 0)
	//for index, char := range newRunes {
	//	positionsInOld, ok := lookupOldRunes[char]
	//	if ok && len(positionsInOld) > 0 {
	//		sameOld = appendAtBottomDiffrerenceLetter(sameOld, differenceStaticLetter{Letter: char, Index: positionsInOld[0]})
	//		sameNew = appendAtBottomDiffrerenceLetter(sameNew, differenceStaticLetter{Letter: char, Index: index})
	//		lookupOldRunes[char] = positionsInOld[1:]
	//	}
	//}
	//sameOld := make([]webtools.KeyValuePair[rune, int], 0)
	////foundByA := make(map[rune]int, 0)
	//for keyA, valA := range oldRunes {
	//	sameOld = append(sameOld, webtools.KeyValuePair[rune, int]{Key: valA, Value: keyA})
	//}
	//sameNew := make([]webtools.KeyValuePair[rune, int], 0)
	//for keyA, valA := range newRunes {
	//	sameNew = append(sameNew, webtools.KeyValuePair[rune, int]{Key: valA, Value: keyA})
	//}
	//fmt.Println("Same letters old:", sameOld, len(oldRunes)-len(sameOld))
	//fmt.Println("Same letters new:", sameNew)

	//Make LCS diff matrix
	//OLD -> Placed in row = Identifies COLUMN -> X
	//NEW -> Placed in column = Identifies ROW -> Y
	//MATRIX[y = ROW][x = COLUMN]
	matrix := make([][]int, len(new))
	for i := 0; i < len(matrix); i++ {
		matrix[i] = make([]int, len(old))
	}

	//Fill matrix
	//linkedColToRow := make([]int, len(sameNew))
	//for i := 0; i < len(linkedColToRow); i++ {
	//	linkedColToRow[i] = -1
	//}
	for y := 0; y < len(matrix); y++ {
		for x := 0; x < len(matrix[y]); x++ {
			if old[x] == new[y] {
				//Values same
				//linkedColToRow[x] = y
				if y < 1 || x < 1 {
					matrix[y][x] = 1
				} else {
					matrix[y][x] = matrix[y-1][x-1] + 1
				}
				continue
			}

			//Values not same
			a := 0
			if x > 0 {
				a = matrix[y][x-1]
			}
			b := 0
			if y > 0 {
				b = matrix[y-1][x]
			}
			matrix[y][x] = max(a, b)
		}
	}
	//fmt.Println("Matrix:")
	//for y := 0; y < len(matrix); y++ {
	//	for x := 0; x < len(matrix[y]); x++ {
	//		fmt.Print(matrix[y][x])
	//	}
	//	fmt.Println()
	//}
	//fmt.Println("Linked:")
	//fmt.Println(sameOld)
	//for i := 0; i < len(linkedColToRow); i++ {
	//	fmt.Print(linkedColToRow[i])
	//}
	//fmt.Println()
	//fmt.Println()

	//Backtrack the matrix
	//OLD -> COLUMN
	//NEW -> ROW
	//MATRIX[y = ROW][x = COLUMN]
	matrixResults := make([]webtools.ThreeValuePair[T, int, int], 0)
	//matrixResultLinking :=
	y := len(matrix) - 1
	x := len(matrix[y]) - 1
	for true {
		if x < 0 || y < 0 {
			//No more values
			break
		}
		if new[y] == old[x] {
			//Same letters
			linked := webtools.ThreeValuePair[T, int, int]{A: old[x], B: x, C: y}
			matrixResults = append(matrixResults, linked)
			x--
			y--
			continue
		}
		if x == 0 {
			//Wait for last value
			y--
			continue
		}
		if y == 0 {
			//Wait for last value
			x--
			continue
		}

		//Step back
		if matrix[y-1][x] >= matrix[y][x-1] {
			y--
		} else {
			x--
		}
	}
	slices.Reverse(matrixResults)
	//fmt.Println("Used memory:", (len(old) * len(new) * 8), "bytes")
	//fmt.Println("Backtracked matrix:", matrixResults)

	//Detect insertions and deletions
	//resultDelete := make([]DifferenceEntry, 0)
	//resultAdd := make([]DifferenceEntry, 0)
	result := make([]DifferenceEntry[T], 0)
	okOld := 0
	okNew := 0
	checkedLast := false
	for _, matrixItem := range matrixResults {
		if okOld < matrixItem.B {
			//All items before this are invalid
			for _ = okOld; okOld < matrixItem.B; okOld++ {
				result = append(result, DifferenceEntry[T]{Position: okOld + offset, Character: old[okOld], IsInsertion: false})
			}
			okOld++
		} else {
			okOld = matrixItem.B + 1
		}
		if okNew < matrixItem.C {
			//All items before this are new
			for _ = okNew; okNew < matrixItem.C; okNew++ {
				//resultAdd = append([]webtools.ThreeValuePair[int, rune, bool]{{A: okNew, B: rune(newRunes[okNew]), C: true}}, resultAdd...)
				result = append(result, DifferenceEntry[T]{Position: okNew + offset, Character: new[okNew], IsInsertion: true})
			}
			okNew++
		} else {
			okNew = matrixItem.C + 1
		}
		if okNew == len(new) {
			checkedLast = true
		}
	}

	//Do last insertions and deletions
	for i := len(old) - 1; i >= okOld; i-- {
		result = append(result, DifferenceEntry[T]{Position: i + offset, Character: old[i], IsInsertion: false})
	}
	for i := okNew + webtools.FormatByBool(checkedLast, 1, 0); i < len(new); i++ {
		result = append(result, DifferenceEntry[T]{Position: i + offset, Character: new[i], IsInsertion: true})
	}

	return result
}

/*
DiffInStringLCSAlt checks for differences in two arrays. Returns array of changes
It handles lots of changes the best. Uses uint32 for values, because for larger values it will take ages to complete
Trying custom LCS diff check with more effective RAM usage - https://en.wikipedia.org/wiki/Longest_common_subsequence
*/
func DiffInStringLCSAlt[T comparable](old []T, new []T) []DifferenceEntry[T] {
	//Check if not empty
	old, new, offset := DiffTrim(old, new)
	if len(old) == 0 && len(new) == 0 {
		return nil
	}

	//Make LCS diff matrix but only 2 rpws
	//OLD -> Placed in row = Identifies COLUMN -> X
	//NEW -> Placed in column = Identifies ROW -> Y
	//MATRIX[y = ROW][x = COLUMN]
	matrix := make([][]uint32, 2)
	matrix[0] = make([]uint32, len(old))
	matrix[1] = make([]uint32, len(old))
	matrixValuesByLetterValues := make([][]uint64, 0)
	var jumpValue uint32 = 0

	//Fill matrix
	//fmt.Println("Matrix:")
	for y := 0; y < len(new); y++ {
		for x := 0; x < len(old); x++ {
			if old[x] == new[y] {
				//Values same - create row if needed
				//if len(matrixValuesByLetterValues) <= y {
				//	matrixValuesByLetterValues = append(matrixValuesByLetterValues, make([]webtools.KeyValuePair[int, int], 0))
				//}

				//Set value
				if y < 1 || x < 1 {
					matrix[1][x] = 1
				} else {
					matrix[1][x] = matrix[0][x-1] + 1
				}
				if matrix[1][x] > jumpValue {
					jumpValue = matrix[1][x]
				}

				//Values same - create row if needed
				if uint32(len(matrixValuesByLetterValues)) < matrix[1][x] {
					matrixValuesByLetterValues = append(matrixValuesByLetterValues, nil)
				}
				if uint32(len(matrixValuesByLetterValues)) == matrix[1][x] {
					matrixValuesByLetterValues = append(matrixValuesByLetterValues, make([]uint64, 0))
				}

				//Add entry in matrixValues
				if matrix[1][x] >= jumpValue {
					matrixValuesByLetterValues[jumpValue] = append(matrixValuesByLetterValues[jumpValue], ((uint64(x) << 32) | (uint64(y) & 0xFFFFFFFF)))
				}
				continue
			}

			//Values not same
			var a uint32 = 0
			if x > 0 {
				a = matrix[1][x-1]
			}
			var b uint32 = 0
			if y > 0 {
				b = matrix[0][x]
			}
			matrix[1][x] = max(a, b)
		}

		//Values same - create empty row
		if len(matrixValuesByLetterValues) <= y {
			matrixValuesByLetterValues = append(matrixValuesByLetterValues, nil)
		}

		//for x := 0; x < len(matrix[1]); x++ {
		//	fmt.Print(matrix[1][x])
		//}
		//fmt.Println()

		//Shift row
		copy(matrix[0], matrix[1])
	}
	//fmt.Println("Matrix:")
	//for y := 0; y < len(matrix); y++ {
	//	for x := 0; x < len(matrix[y]); x++ {
	//		fmt.Print(matrix[y][x])
	//	}
	//	fmt.Println()
	//}
	//fmt.Println("Filtered values:", matrixValuesByRows)
	//fmt.Println("Linked:")
	//fmt.Println(sameOld)
	//for i := 0; i < len(linkedColToRow); i++ {
	//	fmt.Print(linkedColToRow[i])
	//}
	//fmt.Println()
	//fmt.Println()
	//usedRamCounter := 0
	//for _, v := range matrixValuesByLetterValues {
	//	usedRamCounter += len(v) * 8
	//}

	//Backtrack the matrix
	//OLD -> Placed in row = Identifies COLUMN -> X
	//NEW -> Placed in column = Identifies ROW -> Y
	//MATRIX[y = ROW][x = COLUMN]
	matrixResults := make([]webtools.ThreeValuePair[T, int, int], 0)
	//matrixResultLinking :=
	x := uint32(len(old) - 1)
	y := uint32(len(new) - 1)
	if len(old) > 0 && len(new) > 0 {
		for letterCount := matrix[1][x]; letterCount > 0; letterCount-- {
			//Multiple values, filter area
			matrixValuesByLetterValues[letterCount] = slices.DeleteFunc(matrixValuesByLetterValues[letterCount], func(element uint64) bool {
				return uint32(element>>32) > x || uint32(element&0xFFFFFFFF) > y
			})

			//Check if letter count has only one letter
			if len(matrixValuesByLetterValues[letterCount]) == 1 {
				//Only value, jump to it
				pointX := uint32(matrixValuesByLetterValues[letterCount][0] >> 32)
				pointY := uint32(matrixValuesByLetterValues[letterCount][0] & 0xFFFFFFFF)
				matrixResults = append(matrixResults, webtools.ThreeValuePair[T, int, int]{A: old[pointX], B: int(pointX), C: int(pointY)})
				x = pointX - 1
				y = pointY - 1
				continue
			}

			//Select the most right (right has a priority) and the most bottom value
			bestPointX := uint32(matrixValuesByLetterValues[letterCount][0] >> 32)
			bestPointY := uint32(matrixValuesByLetterValues[letterCount][0] & 0xFFFFFFFF)
			for i := 1; i < len(matrixValuesByLetterValues[letterCount]); i++ {
				pointX := uint32(matrixValuesByLetterValues[letterCount][i] >> 32)
				pointY := uint32(matrixValuesByLetterValues[letterCount][i] & 0xFFFFFFFF)
				if bestPointX < pointX || bestPointX == pointX && bestPointY < pointY {
					//Value of point is more near to current (x,y) point
					bestPointX = pointX
					bestPointY = pointY
					continue
				}

			}

			//Jump to best point
			matrixResults = append(matrixResults, webtools.ThreeValuePair[T, int, int]{A: old[bestPointX], B: int(bestPointX), C: int(bestPointY)})
			x = bestPointX - 1
			y = bestPointY - 1
		}
		slices.Reverse(matrixResults)
	}
	//fmt.Println("Used memory:", (len(old)*2*4)+usedRamCounter, "bytes")
	//fmt.Println("Backtracked matrix:", matrixResults)

	//Detect insertions and deletions
	//resultDelete := make([]DifferenceEntry, 0)
	//resultAdd := make([]DifferenceEntry, 0)
	result := make([]DifferenceEntry[T], 0)
	okOld := 0
	okNew := 0
	checkedLast := false
	for _, matrixItem := range matrixResults {
		if okOld < matrixItem.B {
			//All items before this are invalid
			for _ = okOld; okOld < matrixItem.B; okOld++ {
				//resultDelete = appendAtTop(resultDelete, DifferenceEntry{Position: okOld, Character: rune(oldRunes[okOld]), IsInsertion: false})
				result = append(result, DifferenceEntry[T]{Position: okOld + offset, Character: old[okOld], IsInsertion: false})
			}
			okOld++
		} else {
			okOld = matrixItem.B + 1
		}
		if okNew < matrixItem.C {
			//All items before this are new
			for _ = okNew; okNew < matrixItem.C; okNew++ {
				//resultAdd = append([]webtools.ThreeValuePair[int, rune, bool]{{A: okNew, B: rune(newRunes[okNew]), C: true}}, resultAdd...)
				result = append(result, DifferenceEntry[T]{Position: okNew + offset, Character: new[okNew], IsInsertion: true})
			}
			okNew++
		} else {
			okNew = matrixItem.C + 1
		}
		if okNew == len(new) {
			checkedLast = true
		}
	}

	//Do last insertions and deletions
	for i := len(old) - 1; i >= okOld; i-- {
		result = append(result, DifferenceEntry[T]{Position: i + offset, Character: old[i], IsInsertion: false})
	}
	for i := okNew + webtools.FormatByBool(checkedLast, 1, 0); i < len(new); i++ {
		result = append(result, DifferenceEntry[T]{Position: i + offset, Character: new[i], IsInsertion: true})
	}

	return result
}

///*
//DiffInStringMyers checks for differences in two string without the matrix. Returns array of changes
//Meyers algorithm - https://blog.robertelder.org/diff-algorithm/
//*/
//func DiffInStringMyers(old string, new string) []DifferenceEntry {
//	//Get same letters
//	oldRunes := []rune(old)
//	newRunes := []rune(new)
//
//	//Do Myers simulation
//	//OLD -> Placed in row = Identifies COLUMN -> X
//	//NEW -> Placed in column = Identifies ROW -> Y
//	ended := false
//	kLinesForX := make(map[int]map[int]int)
//	kLinesForX[-1] = make(map[int]int)
//	kLinesForX[-1][1] = 0
//	matrixResults := make([]webtools.ThreeValuePair[rune, int, int], 0)
//	for differences := 0; differences <= (len(oldRunes) + len(newRunes)); differences++ {
//		kLinesForX[differences] = make(map[int]int, 0)
//		//Tells how many edits can be done
//		for kLineIndexCounter := -differences; kLineIndexCounter <= differences; kLineIndexCounter += 2 {
//			//Tells on what diagonal we are - we need to get one back
//			kLineIndex := kLineIndexCounter
//			if kLineIndex <= 0 {
//				kLineIndex++
//			} else {
//				kLineIndex--
//			}
//			//The x is the furthes x we got on this diagonal line
//			get, ok := kLinesForX[differences-1]
//			if !ok {
//				fmt.Println("KLine for differences:", differences, "not reachable.")
//				continue
//			}
//			x, ok := get[kLineIndex]
//			if !ok {
//				fmt.Println("KLine for differences:", differences, " and kLine:", kLineIndex, "not reachable.")
//				continue
//			}
//
//			//The y is calculated from the diagonal definition k = x - y
//			y := x - kLineIndex
//			fmt.Println("Simulating at point: [", x, ",", y, "] at kLine:", kLineIndex, "with differences:", differences)
//			if x < 0 || y < -1 {
//				fmt.Println("Point out of range: [", x, ",", y, "] at kLine:", kLineIndex, "with differences:", differences)
//				continue
//			}
//
//			//Check if can do diagonal, and if can, do it as much as you can
//			if x < len(oldRunes) && y < len(newRunes) && y >= 0 && oldRunes[x] == newRunes[y] {
//				for x < len(oldRunes) && y < len(newRunes) && oldRunes[x] == newRunes[y] {
//					fmt.Println("Moved diagonally from: [", x, ",", y, "] to: [", x+1, ",", y+1, "] at kLine:", kLineIndex, "with differences:", differences)
//					x++
//					y++
//				}
//				kLinesForX[differences][kLineIndex] = x
//
//				//Check if ended
//				if x >= len(oldRunes) && y >= len(newRunes) {
//					ended = true
//					break
//				}
//				continue
//			}
//
//			//Check for moving left or down
//			if (kLineIndex-1) == -differences && y < len(newRunes) {
//				//Can go down
//				localX := x
//				localY := y + 1
//				fmt.Println("Moved down from: [", x, ",", y, "] to: [", localX, ",", localY, "] at kLine:", kLineIndex, "with differences:", differences)
//				for localX < len(oldRunes) && localY < len(newRunes) && oldRunes[localX] == newRunes[localY] {
//					fmt.Println("Moved diagonally from: [", localX, ",", localY, "] to: [", localX+1, ",", localY+1, "] at kLine:", kLineIndex, "with differences:", differences)
//					localX++
//					localY++
//				}
//				kLinesForX[differences][kLineIndex-1] = localX
//
//				//Check if ended
//				if localX >= len(oldRunes) && localY >= len(newRunes) {
//					ended = true
//					break
//				}
//			}
//			if (kLineIndex+1) == differences && x < len(oldRunes) {
//				//Can go left
//				localX := x + 1
//				localY := y
//				fmt.Println("Moved left from: [", x, ",", y, "] to: [", localX, ",", localY, "] at kLine:", kLineIndex, "with differences:", differences)
//				for localX < len(oldRunes) && localY < len(newRunes) && oldRunes[localX] == newRunes[localY] {
//					fmt.Println("Moved diagonally from: [", localX, ",", localY, "] to: [", localX+1, ",", localY+1, "] at kLine:", kLineIndex, "with differences:", differences)
//					localX++
//					localY++
//				}
//				kLinesForX[differences][kLineIndex+1] = localX
//
//				//Check if ended
//				if localX >= len(oldRunes) && localY >= len(newRunes) {
//					ended = true
//					break
//				}
//			}
//		}
//		if ended {
//			break
//		}
//	}
//
//	fmt.Println(kLinesForX)
//	fmt.Println("Backtracked matrix:", matrixResults)
//
//	//No same letters
//	if len(matrixResults) == 0 {
//		//Source string do not match at all
//		result := make([]DifferenceEntry, 0)
//		for i := 0; i < len(oldRunes); i++ {
//			result = webtools.InsertElementAtIndex(result, 0, DifferenceEntry{Position: i, Character: rune(oldRunes[i]), IsInsertion: false})
//		}
//		for i := 0; i < len(newRunes); i++ {
//			result = append(result, DifferenceEntry{Position: i, Character: rune(newRunes[i]), IsInsertion: true})
//		}
//		return result
//	}
//
//	//Detect insertions and deletions
//	resultDelete := make([]DifferenceEntry, 0)
//	resultAdd := make([]DifferenceEntry, 0)
//	okOld := 0
//	okNew := 0
//	checkedLast := false
//	for _, matrixItem := range matrixResults {
//		if okOld < matrixItem.B {
//			//All items before this are invalid
//			for _ = okOld; okOld < matrixItem.B; okOld++ {
//				resultDelete = appendAtTop(resultDelete, DifferenceEntry{Position: okOld, Character: rune(oldRunes[okOld]), IsInsertion: false})
//			}
//			okOld++
//		} else {
//			okOld = matrixItem.B + 1
//		}
//		if okNew < matrixItem.C {
//			//All items before this are new
//			for _ = okNew; okNew < matrixItem.C; okNew++ {
//				//resultAdd = append([]webtools.ThreeValuePair[int, rune, bool]{{A: okNew, B: rune(newRunes[okNew]), C: true}}, resultAdd...)
//				resultAdd = append(resultAdd, DifferenceEntry{Position: okNew, Character: rune(newRunes[okNew]), IsInsertion: true})
//			}
//			okNew++
//		} else {
//			okNew = matrixItem.C + 1
//		}
//		if okNew == len(newRunes) {
//			checkedLast = true
//		}
//	}
//
//	//Do last insertions and deletions
//	for i := len(oldRunes) - 1; i >= okOld; i-- {
//		resultDelete = appendAtTop(resultDelete, DifferenceEntry{Position: i, Character: rune(oldRunes[i]), IsInsertion: false})
//	}
//	for i := okNew + webtools.FormatByBool(checkedLast, 1, 0); i < len(newRunes); i++ {
//		resultAdd = append(resultAdd, DifferenceEntry{Position: i, Character: rune(newRunes[i]), IsInsertion: true})
//	}
//
//	return append(resultDelete, resultAdd...)
//}

//type diffMyersSimulationPoint struct {
//	X        int
//	Y        int
//	CameFrom *diffMyersSimulationPoint
//	Distance int
//}
//
//func myersDoDiagonal(x int, y int, targetX int, targetY int, oldRunes []rune, newRunes []rune) (int, int, bool) {
//	moved := false
//	for x < targetX && y < targetY && oldRunes[x] == newRunes[y] {
//		//kLinesValues[kLine] = kLinesValues[kLine] + 1
//		fmt.Println("Moved diagonally from:[", x, ",", y, "] to: [", x+1, ",", y+1, "]")
//		x++
//		y++
//		moved = true
//	}
//	return x, y, moved
//}

/*
https://blog.robertelder.org/diff-algorithm/
*/
//func DiffInStringMyers(old string, new string) []DifferenceEntry {
//	oldRunes := []rune(old)
//	newRunes := []rune(new)
//
//	//Prepare Myers simulation
//	//OLD -> Placed in row = Identifies COLUMN -> X
//	//NEW -> Placed in column = Identifies ROW -> Y
//	targetX := len(oldRunes)
//	targetY := len(newRunes)
//	iteration := 0
//	furthestOnKLines := make(map[int]int, 0)
//	backtrace := make(map[int]map[int]webtools.ThreeValuePair[int, int, int], 0)
//	var ended bool
//	var differences int
//
//	//Do simulations
//	for differences = 0; differences <= (len(oldRunes) + len(newRunes)); differences++ {
//		backtrace[differences] = make(map[int]webtools.ThreeValuePair[int, int, int], 0)
//		for kLineIndex := -(differences - 2*max(0, differences-targetX)); kLineIndex <= (differences - 2*max(0, differences-targetY)); kLineIndex += 2 {
//			iteration++
//			var x int
//			var nowX int = furthestOnKLines[kLineIndex]
//			var nowY int = nowX - kLineIndex
//			var prevKLineIndex int
//			fmt.Println("Simulating at point: [", nowX, ",", nowY, "] at kLine:", kLineIndex, "with differences:", differences)
//			//Select direction
//			if kLineIndex == -differences || (kLineIndex != differences && furthestOnKLines[kLineIndex-1] < furthestOnKLines[kLineIndex+1]) {
//				//Must go down
//				x = furthestOnKLines[kLineIndex+1]
//				prevKLineIndex = kLineIndex + 1
//				fmt.Println("Moved left from:[", nowX, ",", nowY, "] to: [", x, ",", nowY+1, "]")
//			} else {
//				//Must go left
//				x = furthestOnKLines[kLineIndex-1] + 1
//				prevKLineIndex = kLineIndex - 1
//				fmt.Println("Moved down from:[", nowX, ",", nowY, "] to: [", x, ",", nowY, "]")
//			}
//			y := x - kLineIndex
//			if y < 0 || x < 0 {
//				fmt.Println("Moved invalidated to: [", x, ",", nowY, "]")
//				continue
//			}
//			if x >= targetX && y >= targetY {
//				ended = true
//				break
//			}
//
//			//Do diagonal
//			for x < targetX && y < targetY && oldRunes[x] == newRunes[y] {
//				//kLinesValues[kLine] = kLinesValues[kLine] + 1
//				fmt.Println("Moved diagonally from:[", x, ",", y, "] to: [", x+1, ",", y+1, "]")
//				x++
//				y++
//			}
//			furthestOnKLines[kLineIndex] = x
//			backtrace[differences][kLineIndex] = webtools.ThreeValuePair[int, int, int]{A: x, B: y, C: prevKLineIndex}
//		}
//		if ended {
//			break
//		}
//	}
//	//Get top simulation point
//	//simPoint := simulationPoints[0]
//	//simulationPoints = simulationPoints[1:]
//	//kLine := simPoint.X - simPoint.Y
//	//
//	//if currentD < simPoint.Distance {
//	//	currentD = simPoint.Distance
//	//}
//	//if currentD > simPoint.Distance {
//	//	fmt.Println("Ignoring point:[", simPoint.X, ",", simPoint.Y, "] with distance:", simPoint.Distance, "; global distance:", currentD)
//	//	continue
//	//}
//
//	//Check if in the end
//	//if simPoint.X == targetX && simPoint.Y == targetY {
//	//	endPoint = simPoint
//	//	break
//	//}
//
//	//Check if can diagonal
//	//newPoint, didDiag := myersDoDiagonal(simPoint, targetX, targetY, oldRunes, newRunes)
//	//if didDiag {
//	//	simulationPoints = append(simulationPoints, newPoint)
//	//	continue
//	//}
//
//	//Check if can go to kLine
//	//if webtools.IntAbs(kLine) > simPoint.Distance {
//	//	fmt.Println("Can not move to:[", simPoint.X, ",", simPoint.Y, "] because of kLine: ", webtools.IntAbs(kLine), " is smaller than: ", simPoint.Distance)
//	//	continue
//	//}
//
//	//Split in two
//	//if simPoint.Y < targetY {
//	//	furthestOnKLines[kLine] = furthestOnKLines[kLine+1]
//	//	fmt.Println("Moved down from:[", simPoint.X, ",", simPoint.Y, "] to: [", simPoint.X, ",", simPoint.Y+1, "]")
//	//	newPoint, _ := myersDoDiagonal(diffMyersSimulationPoint{X: simPoint.X, Y: simPoint.Y + 1, CameFrom: &simPoint, Distance: simPoint.Distance + 1}, targetX, targetY, oldRunes, newRunes)
//	//	simulationPoints = append(simulationPoints, newPoint)
//	//}
//	//if simPoint.X < targetX {
//	//	furthestOnKLines[kLine] = furthestOnKLines[kLine-1] + 1
//	//	fmt.Println("Moved right from:[", simPoint.X, ",", simPoint.Y, "] to: [", simPoint.X+1, ",", simPoint.Y, "]")
//	//	newPoint, _ := myersDoDiagonal(diffMyersSimulationPoint{X: simPoint.X + 1, Y: simPoint.Y, CameFrom: &simPoint, Distance: simPoint.Distance + 1}, targetX, targetY, oldRunes, newRunes)
//	//	simulationPoints = append(simulationPoints, newPoint)
//	//}
//
//	//Print path
//	fmt.Println("Path:", differences)
//	fmt.Println(furthestOnKLines)
//	fmt.Println(backtrace)
//
//	//Backtrace
//	x := targetX
//	y := targetY
//	result := make([]DifferenceEntry, 0)
//	for differences = differences; differences > 0; differences-- {
//		kLineIndex := x - y
//		prevPoint := backtrace[differences][kLineIndex]
//		for i := x - 1; i >= prevPoint.C; i++ {
//			//Matches
//			x--
//			y--
//		}
//		if prevPoint.C == kLineIndex-1 {
//			//Deleted
//			result = append(result, DifferenceEntry{Position: prevPoint.A, Character: oldRunes[prevPoint.A], IsInsertion: false})
//		}
//		if prevPoint.C == kLineIndex+1 {
//			//Inserted
//			result = append(result, DifferenceEntry{Position: prevPoint.B, Character: newRunes[prevPoint.B], IsInsertion: true})
//		}
//		x = prevPoint.A
//		y = prevPoint.B
//	}
//	//var currentPoint *diffMyersSimulationPoint = &endPoint
//	//for currentPoint != nil {
//	//	fmt.Println("X:", currentPoint.X, "; Y:", currentPoint.Y)
//	//	currentPoint = currentPoint.CameFrom
//	//}
//	fmt.Println("Iterations:", iteration)
//	return result
//}

///*
//https://blog.robertelder.org/diff-algorithm/
//*/
//func DiffInStringMyers[T comparable](old []T, new []T) []DifferenceEntry[T] {
//	//Create kLineValues map = stores values of each k line, key calculated by k = x - y
//	kLineValues := []map[int]int{{0: 0}}
//	found := false
//	x := 0
//	y := 0
//	differences := 0
//	kLineIndex := 0
//	result := make([]DifferenceEntry[T], 0)
//
//	//Calculate max number of differences
//	//OLD -> Placed in row = Identifies COLUMN -> X
//	//NEW -> Placed in column = Identifies ROW -> Y
//	for differences = differences; differences <= (len(old) + len(new)); differences++ {
//		//Get and make k line maps
//		var kLineValuesPrevious = kLineValues[differences]
//		var kLineValuesNew = make(map[int]int)
//		//Simulate each k line (can skip because of Myers behavior)
//		for kLineIndex := -differences; kLineIndex <= differences; kLineIndex += 2 {
//			//Get value of kLineValuesOld and set them to kLineValuesNew = simulate
//			if kLineIndex == 0 && differences == 0 {
//				x = 0
//			} else {
//				xAtKMinus, ok := kLineValuesPrevious[kLineIndex-1]
//				if !ok {
//					xAtKMinus = -1
//				}
//				xAtKPlus, ok := kLineValuesPrevious[kLineIndex+1]
//				if !ok {
//					xAtKPlus = -1
//				}
//
//				//Check if can move
//				if kLineIndex == -differences || (kLineIndex != differences && xAtKMinus < xAtKPlus) {
//					//Can go down
//					x = xAtKPlus
//					//fmt.Println("Going down")
//				} else {
//					//Can go right
//					x = xAtKMinus + 1
//					//fmt.Println("Going right")
//				}
//			}
//			//Get y
//			y = x - kLineIndex
//			//fmt.Println("Simulating: [", x, ",", y, "] with k line index:", kLineIndex, "and differences:", differences)
//
//			//Check cases
//			for x < len(old) && y < len(new) && old[x] == new[y] {
//				x++
//				y++
//			}
//
//			kLineValuesNew[kLineIndex] = x
//			//Check for end
//			if x >= len(old) && y >= len(new) {
//				fmt.Println("Simulation ended at: [", x, ",", y, "] with k line index:", kLineIndex, "and differences:", differences)
//				found = true
//				break
//			}
//		}
//
//		//fmt.Println(kLineValuesNew)
//		//Clone data to new map
//		kLineValues = append(kLineValues, kLineValuesNew)
//		if found {
//			break
//		}
//	}
//
//	//Backtrace
//	fmt.Println(kLineValues)
//	fmt.Println(x, y, kLineIndex)
//	//ended := false
//	var prevKLineIndex int
//	for differences = differences; differences > 0; differences-- {
//		kLineIndex = x - y
//		if kLineIndex == -differences {
//			//Moved up
//			prevKLineIndex = kLineIndex + 1
//		} else if kLineIndex == differences {
//			//Moved left
//			prevKLineIndex = kLineIndex - 1
//		} else if kLineValues[differences-1][kLineIndex-1] < kLineValues[differences-1][kLineIndex+1] {
//			//Move up because of bigger value
//			prevKLineIndex = kLineIndex + 1
//		} else {
//			//Move left
//			prevKLineIndex = kLineIndex - 1
//		}
//
//		//Get prevous x
//		prevX := kLineValues[differences-1][prevKLineIndex]
//		prevY := prevX - prevKLineIndex
//
//		//Check for diagonal
//		for x > prevX && y > prevY {
//			//Same, jump up
//			fmt.Println("Moved diagonally from: [", x, ",", y, "] to: [", x-1, ",", y-1, "]")
//			x--
//			y--
//		}
//
//		//Check for insertion
//		if prevKLineIndex == kLineIndex+1 && y > 0 {
//			result = append(result, DifferenceEntry[T]{Position: x - 1, Character: new[y-1], IsInsertion: true})
//			fmt.Println("Moved up from: [", x, ",", y, "] to: [", x, ",", y-1, "]")
//			y--
//		}
//
//		//Check for deletion
//		if prevKLineIndex == kLineIndex-1 && x > 0 {
//			result = append(result, DifferenceEntry[T]{Position: y, Character: old[x-1], IsInsertion: false})
//			fmt.Println("Moved left from: [", x, ",", y, "] to: [", x-1, ",", y, "]")
//			x--
//		}
//		kLineIndex = x - y
//	}
//	return result
//}

/*
PatchUsingChanges patches old array using changes (differences)
*/
func PatchUsingChanges[T comparable](old []T, changes []DifferenceEntry[T]) []T {
	//Filter deletions and insertions and sort them by indexes
	deletions := make([]DifferenceEntry[T], 0)
	insertions := make([]DifferenceEntry[T], 0)
	for _, v := range changes {
		if v.IsInsertion {
			insertions = appendAtBottom(insertions, v)
		} else {
			deletions = appendAtTop(deletions, v)
		}
	}

	//Process
	for _, v := range deletions {
		old = webtools.RemoveElementAtIndex(old, v.Position)
		fmt.Println(old)
	}
	for _, v := range insertions {
		old = webtools.InsertElementAtIndex(old, v.Position, v.Character)
		fmt.Println(old)
	}
	return old
}
