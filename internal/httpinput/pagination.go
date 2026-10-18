package httpinput

import (
	"errors"
	"net/url"
	"strconv"
)

func Pagination(values url.Values) (int, int, error) {
	limit, offset := 50, 0
	if values.Has("limit") {
		parsed, err := strconv.Atoi(values.Get("limit"))
		if err != nil || parsed < 1 || parsed > 100 {
			return 0, 0, errors.New("limit must be between 1 and 100")
		}
		limit = parsed
	}
	if values.Has("offset") {
		parsed, err := strconv.Atoi(values.Get("offset"))
		if err != nil || parsed < 0 || parsed > 1000000 {
			return 0, 0, errors.New("offset must be between 0 and 1000000")
		}
		offset = parsed
	}
	return limit, offset, nil
}
