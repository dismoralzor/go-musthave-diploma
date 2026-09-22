// Package luhn реализует проверку номеров по алгоритму Луна, используемому
// для валидации номеров заказов.
package luhn

// Valid сообщает, состоит ли number только из цифр и проходит ли он
// контрольную сумму по алгоритму Луна. Пустая строка и строки с
// нецифровыми символами считаются невалидными.
func Valid(number string) bool {
	if number == "" {
		return false
	}

	sum := 0
	double := false

	for i := len(number) - 1; i >= 0; i-- {
		c := number[i]
		if c < '0' || c > '9' {
			return false
		}

		digit := int(c - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}

	return sum%10 == 0
}
