package search

// ConvertQwertyToHangul converts QWERTY keystrokes typed with a Korean
// 2-bulshik layout into composed Hangul syllables (C-4). It is a pure,
// general algorithm — not a rewrite dictionary — so any mistyped-layout
// query is convertible. Characters with no layout mapping (digits, spaces,
// already-Hangul text) pass through unchanged.
func ConvertQwertyToHangul(s string) string {
	// ASCII key -> jamo class: 'C' choseong index or 'V' jungseong index.
	type key struct {
		isVowel bool
		index   int
	}
	layout := map[rune]key{
		'q': {false, 7}, 'w': {false, 12}, 'e': {false, 3}, 'r': {false, 0},
		't': {false, 9}, 'a': {false, 6}, 's': {false, 2}, 'd': {false, 11},
		'f': {false, 5}, 'g': {false, 18}, 'z': {false, 15}, 'x': {false, 16},
		'c': {false, 14}, 'v': {false, 17},
		'y': {true, 12}, 'u': {true, 6}, 'i': {true, 2}, 'o': {true, 1},
		'p': {true, 5}, 'h': {true, 8}, 'j': {true, 4}, 'k': {true, 0},
		'l': {true, 20}, 'b': {true, 17}, 'n': {true, 13}, 'm': {true, 18},
		'Q': {false, 8}, 'W': {false, 13}, 'E': {false, 4}, 'R': {false, 1},
		'T': {false, 10}, 'O': {true, 3}, 'P': {true, 7},
	}
	upper := map[rune]rune{
		'Y': 'y', 'U': 'u', 'I': 'i', 'A': 'a', 'S': 's', 'D': 'd',
		'F': 'f', 'G': 'g', 'H': 'h', 'J': 'j', 'K': 'k', 'L': 'l',
		'Z': 'z', 'X': 'x', 'C': 'c', 'V': 'v', 'B': 'b', 'N': 'n', 'M': 'm',
	}
	choJamo := []rune{0x3131, 0x3132, 0x3134, 0x3137, 0x3138, 0x3139, 0x3141, 0x3142, 0x3143, 0x3145, 0x3146, 0x3147, 0x3148, 0x3149, 0x314A, 0x314B, 0x314C, 0x314D, 0x314E}
	// choseong -> jongseong, -1 when the consonant cannot close a syllable.
	choToJong := []int{1, 2, 4, 7, -1, 8, 16, 17, -1, 19, 20, 21, 22, -1, 23, 24, 25, 26, 27}
	// jongseong -> (kept jong, carried choseong) when a vowel follows.
	jongSplit := [28][2]int{
		{0, 11}, {0, 0}, {0, 1}, {1, 9}, {0, 2}, {4, 12}, {4, 18},
		{0, 3}, {0, 5}, {8, 0}, {8, 6}, {8, 7}, {8, 9}, {8, 16},
		{8, 17}, {8, 18}, {0, 6}, {0, 7}, {17, 9}, {0, 9},
		{0, 10}, {0, 11}, {0, 12}, {0, 14}, {0, 15}, {0, 16},
		{0, 17}, {0, 18},
	}
	compoundJung := map[[2]int]int{
		{8, 0}: 9, {8, 1}: 10, {8, 20}: 11,
		{13, 4}: 14, {13, 5}: 15, {13, 20}: 16, {18, 20}: 19,
	}
	compoundJong := map[[2]int]int{
		{1, 19}: 3, {4, 22}: 5, {4, 27}: 6, {8, 1}: 9, {8, 16}: 10,
		{8, 17}: 11, {8, 19}: 12, {8, 25}: 13, {8, 26}: 14, {8, 27}: 15,
		{17, 19}: 18,
	}

	var out []rune
	cho, jung, jong := -1, -1, 0
	flush := func() {
		switch {
		case cho >= 0 && jung >= 0:
			out = append(out, 0xAC00+rune((cho*21+jung)*28+jong))
		case cho >= 0:
			out = append(out, choJamo[cho])
		}
		cho, jung, jong = -1, -1, 0
	}
	for _, r := range s {
		k, ok := layout[r]
		if !ok {
			if lower, isUpper := upper[r]; isUpper {
				k, ok = layout[lower]
			}
		}
		if !ok {
			flush()
			out = append(out, r)
			continue
		}
		if !k.isVowel {
			c := k.index
			switch {
			case cho < 0:
				cho = c
			case jung < 0:
				flush()
				cho = c
			case jong == 0:
				if j := choToJong[c]; j >= 0 {
					jong = j
				} else {
					flush()
					cho = c
				}
			default:
				prevJong := jong
				if j, ok := compoundJong[[2]int{prevJong, choToJong[c]}]; ok && choToJong[c] >= 0 {
					jong = j
				} else {
					flush()
					cho = c
				}
			}
			continue
		}
		v := k.index
		switch {
		case cho < 0:
			cho, jung = 11, v // leading ㅇ
		case jung < 0:
			jung = v
		case jong == 0:
			if j, ok := compoundJung[[2]int{jung, v}]; ok {
				jung = j
			} else {
				flush()
				cho, jung = 11, v
			}
		default:
			kept, carried := jongSplit[jong][0], jongSplit[jong][1]
			jong = kept
			flush()
			cho, jung = carried, v
		}
	}
	flush()
	return string(out)
}
