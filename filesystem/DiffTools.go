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
			for pos = pos; pos < len(new); pos++ {
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
		for pos = pos; pos < len(old); pos++ {
			result = append(result, webtools.KeyValuePair[int, FileSystemEventType]{Key: pos, Value: FSEventDeleted})
		}
	}
	return result
}

/*
Simplified Myers diff check - https://en.wikipedia.org/wiki/Longest_common_subsequence
*/
func ChangesInString(old string, new string) []webtools.ThreeValuePair[int, rune, bool] {
	//Get same letters
	sameOld := make([]webtools.KeyValuePair[rune, int], 0)
	linkedCheck := make([]bool, len(new))
	for keyA, valA := range old {
		for keyB, valB := range new {
			if valA == valB && !linkedCheck[keyB] {
				linkedCheck[keyB] = true
				sameOld = append(sameOld, webtools.KeyValuePair[rune, int]{Key: valA, Value: keyA})
				break
			}
		}
	}
	sameNew := make([]webtools.KeyValuePair[rune, int], 0)
	for k, v := range linkedCheck {
		if v {
			sameNew = append(sameNew, webtools.KeyValuePair[rune, int]{Key: rune(new[k]), Value: k})
		}
	}
	fmt.Println("Same letters old:", sameOld)
	fmt.Println("Same letters new:", sameNew)

	//Make Myers diff matrix
	//OLD -> COLUMN
	//NEW -> ROW
	//MATRIX[y = ROW][x = COLUMN]
	matrix := make([][]int, len(sameNew))
	for i := 0; i < len(matrix); i++ {
		matrix[i] = make([]int, len(sameOld))
	}

	//Fill matrix
	linkedColToRow := make([]int, len(sameNew))
	for i := 0; i < len(linkedColToRow); i++ {
		linkedColToRow[i] = -1
	}
	for y := 0; y < len(matrix); y++ {
		linked := false
		for x := 0; x < len(matrix[y]); x++ {
			if !linked && sameOld[y].Key == sameNew[x].Key && linkedColToRow[x] == -1 {
				//Values same
				linkedColToRow[x] = y
				linked = true
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
			matrix[y][x] = webtools.IntMax(a, b)
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
	fmt.Println("Linked:")
	fmt.Println(sameOld)
	for i := 0; i < len(linkedColToRow); i++ {
		fmt.Print(linkedColToRow[i])
	}
	fmt.Println()
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
		if matrix[y][x-1] > matrix[y-1][x] {
			y--
			continue
		} else {
			x--
			continue
		}
	}
	fmt.Println("Backtracked matrix:", matrixResults)

	//Detect insertions and deletions
	result := make([]webtools.ThreeValuePair[int, rune, bool], 0)
	okOld := 0
	okNew := 0
	for _, matrixItem := range matrixResults {
		if okOld < matrixItem.B {
			//All items before this are invalid
			for _ = okOld; okOld < matrixItem.B; okOld++ {
				result = append(result, webtools.ThreeValuePair[int, rune, bool]{A: okOld, B: rune(old[okOld]), C: false})
			}
			okOld++
		} else {
			okOld = matrixItem.B + 1
		}
		if okNew < matrixItem.C {
			//All items before this are new
			for _ = okNew; okNew < matrixItem.C; okNew++ {
				result = append(result, webtools.ThreeValuePair[int, rune, bool]{A: okNew, B: rune(new[okNew]), C: true})
			}
			okNew++
		} else {
			okNew = matrixItem.C + 1
		}
	}
	return result
}
