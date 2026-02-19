package filesystem

import (
	"fmt"
	"slices"
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

type DifferenceEntry struct {
	Position    int
	Character   rune
	IsInsertion bool
}

func appendAtTop(slice []DifferenceEntry, item DifferenceEntry) []DifferenceEntry {
	//Inserts value from the start - biggest position at the top
	for i := 0; i < len(slice); i++ {
		if slice[i].Position < item.Position {
			return webtools.InsertElementAtIndex(slice, i, item)
		}
	}
	return append(slice, item)
}

func appendAtBottom(slice []DifferenceEntry, item DifferenceEntry) []DifferenceEntry {
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
DiffInStringLCS checks for differences in two string. Returns array of changes
Simplified LCS diff check - https://en.wikipedia.org/wiki/Longest_common_subsequence
*/
func DiffInStringLCS(old string, new string) []DifferenceEntry {
	//Get same letters
	oldRunes := []rune(old)
	newRunes := []rune(new)
	sameOld := make([]differenceStaticLetter, 0)
	lookupOldRunes := make(map[rune][]int, 0)
	for index, char := range oldRunes {
		if lookupOldRunes[char] == nil {
			lookupOldRunes[char] = make([]int, 0)
		}
		lookupOldRunes[char] = append(lookupOldRunes[char], index)
	}

	//Get same letters
	sameNew := make([]differenceStaticLetter, 0)
	for index, char := range newRunes {
		positionsInOld, ok := lookupOldRunes[char]
		if ok && len(positionsInOld) > 0 {
			sameOld = appendAtBottomDiffrerenceLetter(sameOld, differenceStaticLetter{Letter: char, Index: positionsInOld[0]})
			sameNew = appendAtBottomDiffrerenceLetter(sameNew, differenceStaticLetter{Letter: char, Index: index})
			lookupOldRunes[char] = positionsInOld[1:]
		}
	}
	//sameOld := make([]webtools.KeyValuePair[rune, int], 0)
	////foundByA := make(map[rune]int, 0)
	//for keyA, valA := range oldRunes {
	//	sameOld = append(sameOld, webtools.KeyValuePair[rune, int]{Key: valA, Value: keyA})
	//}
	//sameNew := make([]webtools.KeyValuePair[rune, int], 0)
	//for keyA, valA := range newRunes {
	//	sameNew = append(sameNew, webtools.KeyValuePair[rune, int]{Key: valA, Value: keyA})
	//}
	fmt.Println("Same letters old:", sameOld, len(oldRunes)-len(sameOld))
	fmt.Println("Same letters new:", sameNew)

	if len(sameNew) == 0 || len(sameOld) == 0 {
		//Source string do not match at all
		result := make([]DifferenceEntry, 0)
		for i := 0; i < len(oldRunes); i++ {
			result = webtools.InsertElementAtIndex(result, 0, DifferenceEntry{Position: i, Character: rune(oldRunes[i]), IsInsertion: false})
		}
		for i := 0; i < len(newRunes); i++ {
			result = append(result, DifferenceEntry{Position: i, Character: rune(newRunes[i]), IsInsertion: true})
		}
		return result
	}

	//Make LCS diff matrix
	//OLD -> Placed in row = Identifies COLUMN -> X
	//NEW -> Placed in column = Identifies ROW -> Y
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
			if sameOld[x].Letter == sameNew[y].Letter {
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
		if sameNew[y].Letter == sameOld[x].Letter {
			//Same letters
			linked := webtools.ThreeValuePair[rune, int, int]{A: sameOld[x].Letter, B: sameOld[x].Index, C: sameNew[y].Index}
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
	fmt.Println("Backtracked matrix:", matrixResults)

	//Detect insertions and deletions
	resultDelete := make([]DifferenceEntry, 0)
	resultAdd := make([]DifferenceEntry, 0)
	okOld := 0
	okNew := 0
	checkedLast := false
	for _, matrixItem := range matrixResults {
		if okOld < matrixItem.B {
			//All items before this are invalid
			for _ = okOld; okOld < matrixItem.B; okOld++ {
				resultDelete = appendAtTop(resultDelete, DifferenceEntry{Position: okOld, Character: rune(oldRunes[okOld]), IsInsertion: false})
			}
			okOld++
		} else {
			okOld = matrixItem.B + 1
		}
		if okNew < matrixItem.C {
			//All items before this are new
			for _ = okNew; okNew < matrixItem.C; okNew++ {
				//resultAdd = append([]webtools.ThreeValuePair[int, rune, bool]{{A: okNew, B: rune(newRunes[okNew]), C: true}}, resultAdd...)
				resultAdd = append(resultAdd, DifferenceEntry{Position: okNew, Character: rune(newRunes[okNew]), IsInsertion: true})
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
		resultDelete = appendAtTop(resultDelete, DifferenceEntry{Position: i, Character: rune(oldRunes[i]), IsInsertion: false})
	}
	for i := okNew + webtools.FormatByBool(checkedLast, 1, 0); i < len(newRunes); i++ {
		resultAdd = append(resultAdd, DifferenceEntry{Position: i, Character: rune(newRunes[i]), IsInsertion: true})
	}

	return append(resultDelete, resultAdd...)
}

/*
DiffInStringLCSAlt checks for differences in two string without the matrix. Returns array of changes
Simplified LCS diff check - https://en.wikipedia.org/wiki/Longest_common_subsequence
*/
func DiffInStringLCSAlt(old string, new string) []DifferenceEntry {
	//Get same letters
	oldRunes := []rune(old)
	newRunes := []rune(new)

	//Do simulation like LCS diff matrix
	//OLD -> Placed in row = Identifies COLUMN -> X
	//NEW -> Placed in column = Identifies ROW -> Y
	matrixResults := make([]webtools.ThreeValuePair[rune, int, int], 0)
	var x = 0
	var y = 0
	var found = false
	for true {
		if x >= len(oldRunes) || y >= len(newRunes) {
			break
		}
		found = false
		//Do not match, try to find the matching
		for i := x; i < len(newRunes); i++ {
			if oldRunes[i] == newRunes[y] {
				//Values same
				matrixResults = append(matrixResults, webtools.ThreeValuePair[rune, int, int]{A: oldRunes[x], B: i, C: y})
				x = i + 1
				y++
				found = true
				break
			}
		}

		if !found {
			//Value not found
			y++
		}
	}
	fmt.Println("Backtracked matrix:", matrixResults)

	//No same letters
	if len(matrixResults) == 0 {
		//Source string do not match at all
		result := make([]DifferenceEntry, 0)
		for i := 0; i < len(oldRunes); i++ {
			result = webtools.InsertElementAtIndex(result, 0, DifferenceEntry{Position: i, Character: rune(oldRunes[i]), IsInsertion: false})
		}
		for i := 0; i < len(newRunes); i++ {
			result = append(result, DifferenceEntry{Position: i, Character: rune(newRunes[i]), IsInsertion: true})
		}
		return result
	}

	//Detect insertions and deletions
	resultDelete := make([]DifferenceEntry, 0)
	resultAdd := make([]DifferenceEntry, 0)
	okOld := 0
	okNew := 0
	checkedLast := false
	for _, matrixItem := range matrixResults {
		if okOld < matrixItem.B {
			//All items before this are invalid
			for _ = okOld; okOld < matrixItem.B; okOld++ {
				resultDelete = appendAtTop(resultDelete, DifferenceEntry{Position: okOld, Character: rune(oldRunes[okOld]), IsInsertion: false})
			}
			okOld++
		} else {
			okOld = matrixItem.B + 1
		}
		if okNew < matrixItem.C {
			//All items before this are new
			for _ = okNew; okNew < matrixItem.C; okNew++ {
				//resultAdd = append([]webtools.ThreeValuePair[int, rune, bool]{{A: okNew, B: rune(newRunes[okNew]), C: true}}, resultAdd...)
				resultAdd = append(resultAdd, DifferenceEntry{Position: okNew, Character: rune(newRunes[okNew]), IsInsertion: true})
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
		resultDelete = appendAtTop(resultDelete, DifferenceEntry{Position: i, Character: rune(oldRunes[i]), IsInsertion: false})
	}
	for i := okNew + webtools.FormatByBool(checkedLast, 1, 0); i < len(newRunes); i++ {
		resultAdd = append(resultAdd, DifferenceEntry{Position: i, Character: rune(newRunes[i]), IsInsertion: true})
	}

	return append(resultDelete, resultAdd...)
}

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

/*
PatchUsingChanges patches old string using changes (differences)
*/
func PatchUsingChanges(old string, changes []DifferenceEntry) string {
	//Filter deletions and insertions and sort them by indexes
	deletions := make([]DifferenceEntry, 0)
	insertions := make([]DifferenceEntry, 0)
	for _, v := range changes {
		if v.IsInsertion {
			insertions = appendAtBottom(insertions, v)
		} else {
			deletions = appendAtTop(deletions, v)
		}
	}

	//Process
	for _, v := range deletions {
		old = webtools.RemoveRuneAtIndex(old, v.Position)
		fmt.Println(old)
	}
	for _, v := range insertions {
		old = webtools.InsertRuneAtIndex(old, v.Position, v.Character)
		fmt.Println(old)
	}
	return old
}
