package filesystem

import (
	"fmt"
	"webtools"
)

/*
 */
func ChangesInSequence(old []byte, new []byte) []webtools.KeyValuePair[int, FileSystemEventType] {
	result := make([]webtools.KeyValuePair[int, FileSystemEventType], 0)
	var pos int = 0
	for pos = 0; pos < len(new); pos++ {
		if len(old) <= pos {
			//All after is added
			for _ = pos; pos < len(new); pos++ {
				result = append(result, webtools.KeyValuePair[int, FileSystemEventType]{Key: pos, Value: FSEventCreated})
			}
			break
		}
		if old[pos] != new[pos] {
			result = append(result, webtools.KeyValuePair[int, FileSystemEventType]{Key: pos, Value: FSEventModified})
		}
	}
	if len(old) > pos {
		//All after is removed
		for _ = pos; pos < len(old); pos++ {
			result = append(result, webtools.KeyValuePair[int, FileSystemEventType]{Key: pos, Value: FSEventDeleted})
		}
	}
	return result
}

func appendAtTop(slice []webtools.ThreeValuePair[int, rune, bool], item webtools.ThreeValuePair[int, rune, bool]) []webtools.ThreeValuePair[int, rune, bool] {
	for i := 0; i < len(slice); i++ {
		if slice[i].A < item.A {
			return webtools.InsertElementAtIndex(slice, i, item)
		}
	}
	return append(slice, item)
}

/*
Simplified LCS diff check - https://en.wikipedia.org/wiki/Longest_common_subsequence
*/
func DiffInString(old string, new string) []webtools.ThreeValuePair[int, rune, bool] {
	//Get same letters
	oldRunes := []rune(old)
	newRunes := []rune(new)
	sameOld := make([]webtools.KeyValuePair[rune, int], 0)
	foundByA := make(map[rune]int, 0)
	for keyA, valA := range oldRunes {
		for _, valB := range newRunes {
			if valA == valB {
				foundByA[valB]++
				sameOld = append(sameOld, webtools.KeyValuePair[rune, int]{Key: valA, Value: keyA})
				break
			}
		}
	}
	sameNew := make([]webtools.KeyValuePair[rune, int], 0)
	for k, valB := range newRunes {
		if foundByA[valB] > 0 {
			foundByA[valB]--
			sameNew = append(sameNew, webtools.KeyValuePair[rune, int]{Key: valB, Value: k})
		}
	}
	fmt.Println("Same letters old:", sameOld)
	fmt.Println("Same letters new:", sameNew)

	if len(sameNew) == 0 || len(sameOld) == 0 {
		//Source string do not match at all
		result := make([]webtools.ThreeValuePair[int, rune, bool], 0)
		for i := 0; i < len(oldRunes); i++ {
			result = append(result, webtools.ThreeValuePair[int, rune, bool]{A: i, B: rune(oldRunes[i]), C: false})
		}
		for i := 0; i < len(newRunes); i++ {
			result = append(result, webtools.ThreeValuePair[int, rune, bool]{A: i, B: rune(newRunes[i]), C: true})
		}
		return result
	}

	//Make Myers diff matrix
	//OLD -> COLUMN
	//NEW -> ROW
	//MATRIX[y = ROW][x = COLUMN]
	matrix := make([][]int, len(sameNew))
	for i := 0; i < len(matrix); i++ {
		matrix[i] = make([]int, len(sameOld))
	}

	//Fill matrix
	//linkedColToRow := make([]int, len(sameNew))
	//for i := 0; i < len(linkedColToRow); i++ {
	//	linkedColToRow[i] = -1
	//}
	for y := 0; y < len(matrix); y++ {
		for x := 0; x < len(matrix[y]); x++ {
			if sameOld[x].Key == sameNew[y].Key {
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
	fmt.Println("Matrix:")
	for y := 0; y < len(matrix); y++ {
		for x := 0; x < len(matrix[y]); x++ {
			fmt.Print(matrix[y][x])
		}
		fmt.Println()
	}
	fmt.Println()
	//fmt.Println("Linked:")
	//fmt.Println(sameOld)
	//for i := 0; i < len(linkedColToRow); i++ {
	//	fmt.Print(linkedColToRow[i])
	//}
	//fmt.Println()
	fmt.Println()

	//Backtrack the matrix
	//OLD -> COLUMN
	//NEW -> ROW
	//MATRIX[y = ROW][x = COLUMN]
	matrixResults := make([]webtools.ThreeValuePair[rune, int, int], 0)
	//matrixResultLinking :=
	y := len(matrix) - 1
	x := len(matrix[y]) - 1
	for true {
		if x < 0 || y < 0 {
			//No more values
			break
		}
		if sameNew[y].Key == sameOld[x].Key {
			//Same letters
			linked := webtools.ThreeValuePair[rune, int, int]{A: sameOld[x].Key, B: sameOld[x].Value, C: sameNew[y].Value}
			matrixResults = append([]webtools.ThreeValuePair[rune, int, int]{linked}, matrixResults...)
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
	fmt.Println("Backtracked matrix:", matrixResults)

	//Detect insertions and deletions
	resultDelete := make([]webtools.ThreeValuePair[int, rune, bool], 0)
	resultAdd := make([]webtools.ThreeValuePair[int, rune, bool], 0)
	okOld := 0
	okNew := 0
	checkedLast := false
	for _, matrixItem := range matrixResults {
		if okOld < matrixItem.B {
			//All items before this are invalid
			for _ = okOld; okOld < matrixItem.B; okOld++ {
				resultDelete = appendAtTop(resultDelete, webtools.ThreeValuePair[int, rune, bool]{A: okOld, B: rune(oldRunes[okOld]), C: false})
			}
			okOld++
		} else {
			okOld = matrixItem.B + 1
		}
		if okNew < matrixItem.C {
			//All items before this are new
			for _ = okNew; okNew < matrixItem.C; okNew++ {
				//resultAdd = append([]webtools.ThreeValuePair[int, rune, bool]{{A: okNew, B: rune(newRunes[okNew]), C: true}}, resultAdd...)
				resultAdd = append(resultAdd, webtools.ThreeValuePair[int, rune, bool]{A: okNew, B: rune(newRunes[okNew]), C: true})
			}
			okNew++
		} else {
			okNew = matrixItem.C + 1
		}
		if okNew == len(newRunes) {
			checkedLast = true
		}
	}

	//Do last insertions and deletions
	for i := len(oldRunes) - 1; i >= okOld; i-- {
		resultDelete = appendAtTop(resultDelete, webtools.ThreeValuePair[int, rune, bool]{A: i, B: rune(oldRunes[i]), C: false})
	}
	for i := okNew + webtools.FormatByBool(checkedLast, 1, 0); i < len(newRunes); i++ {
		resultAdd = append(resultAdd, webtools.ThreeValuePair[int, rune, bool]{A: i, B: rune(newRunes[i]), C: true})
	}

	return append(resultDelete, resultAdd...)
}

func UpdateOldUsingChanges(old string, changes []webtools.ThreeValuePair[int, rune, bool]) string {
	for _, v := range changes {
		if !v.C {
			old = webtools.RemoveRuneAtIndex(old, v.A)
		} else {
			old = webtools.InsertRuneAtIndex(old, v.A, v.B)
		}
		fmt.Println(old)
	}
	return old
}
