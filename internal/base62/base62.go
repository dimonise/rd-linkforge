package base62

import (
	"errors"
	"strings"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var (
	ErrEmpty            = errors.New("base62: empty code")
	ErrInvalidCharacter = errors.New("base62: invalid character")
	ErrOverflow         = errors.New("base62: code is too long")
)

// Енкодим из uint64 в base62 string и если число получется больше, чем по списку alphabet,
// то цикл пройдет несколько раз, собираем в слайс и разворачиваем его в конце,
// чтобы получить правильный порядок символов
func Encode(val uint64) string {
	if val == 0 {
		return "0"
	}

	var result []byte

	for val > 0 {
		remainder := val % 62
		result = append(result, alphabet[remainder])
		val /= 62
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// Преобразуем число в base62 строку с заданной шириной. Если закодированная строка меньше заданной ширины,
// то добавляем перед ней нули
func EncodeWidth(val uint64, width int) string {
	encoded := Encode(val)

	if len(encoded) >= width {
		return encoded
	}

	return strings.Repeat("0", width-len(encoded)) + encoded
}

// Декодим base62 строку обратно в uint64.
// Если строка пустая, содержит недопустимые символы или слишком длинная, возвращаем ошибку
func Decode(str string) (uint64, error) {
	if str == "" {
		return 0, ErrEmpty
	}

	if len(str) > 11 {
		return 0, ErrOverflow
	}

	var result uint64

	for _, char := range str {
		index := strings.IndexRune(alphabet, char)

		if index == -1 {
			return 0, ErrInvalidCharacter
		}

		result = result*62 + uint64(index)
	}

	return result, nil
}
