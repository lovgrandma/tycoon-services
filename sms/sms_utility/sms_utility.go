package sms_utility

import (
	"strconv"
)

func main() {

}

func ResolveKey(to string, from string) string {
	if len(to) > 0 && len(from) > 0 {
		var toRaw string = to
		var fromRaw string = from
		if to[0:1] == "+" {
			toRaw = to[1:]
		}
		if from[0:1] == "+" {
			fromRaw = from[1:]
		}
		toVal, errTo := strconv.Atoi(toRaw)
		fromVal, errFrom := strconv.Atoi(fromRaw)
		if errTo != nil || errFrom != nil {
			return ""
		}
		if fromVal < toVal {
			return fromRaw + toRaw
		}
		return toRaw + fromRaw
	}
	return ""
}
