package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrInvalidMoneyFormat = errors.New("invalid money format, expected decimal string with 2 decimal places")
	ErrCurrencyMismatch   = errors.New("currency mismatch between money objects")
	ErrNegativeAmount     = errors.New("negative amount is not allowed for external financial operations")
	ErrOverflow           = errors.New("numeric overflow detected in money operation")
)

var decimalRegex = regexp.MustCompile(`^-?[0-9]+\.[0-9]{2}$`)

type Money struct {
	amount   int64
	currency string
}

func NewMoney(amountStr string, currency string) (Money, error) {
	amountStr = strings.TrimSpace(amountStr)

	if amountStr == "" || strings.EqualFold(amountStr, "NaN") || strings.ContainsAny(amountStr, "eE") {
		return Money{}, ErrInvalidMoneyFormat
	}

	if !decimalRegex.MatchString(amountStr) {
		return Money{}, ErrInvalidMoneyFormat
	}

	parts := strings.Split(amountStr, ".")
	if len(parts) != 2 {
		return Money{}, ErrInvalidMoneyFormat
	}

	intPart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Money{}, ErrOverflow
	}

	fracPart, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || len(parts[1]) != 2 {
		return Money{}, ErrInvalidMoneyFormat
	}

	var totalCents int64
	if intPart < 0 {
		totalCents = (intPart * 100) - fracPart
	} else {
		totalCents = (intPart * 100) + fracPart
	}

	if intPart > math.MaxInt64/100 || intPart < math.MinInt64/100 {
		return Money{}, ErrOverflow
	}

	return Money{
		amount:   totalCents,
		currency: strings.ToUpper(strings.TrimSpace(currency)),
	}, nil
}

func Zero(currency string) Money {
	return Money{
		amount:   0,
		currency: strings.ToUpper(strings.TrimSpace(currency)),
	}
}

func (m Money) Amount() int64 {
	return m.amount
}

func (m Money) Currency() string {
	return m.currency
}

func (m Money) IsZero() bool {
	return m.amount == 0
}

func (m Money) IsPositive() bool {
	return m.amount > 0
}

func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}
	if (other.amount > 0 && m.amount > math.MaxInt64-other.amount) ||
		(other.amount < 0 && m.amount < math.MinInt64-other.amount) {
		return Money{}, ErrOverflow
	}
	return Money{
		amount:   m.amount + other.amount,
		currency: m.currency,
	}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}
	if (other.amount < 0 && m.amount > math.MaxInt64+other.amount) ||
		(other.amount > 0 && m.amount < math.MinInt64+other.amount) {
		return Money{}, ErrOverflow
	}
	return Money{
		amount:   m.amount - other.amount,
		currency: m.currency,
	}, nil
}

func (m Money) MarshalJSON() ([]byte, error) {
	abs := m.amount
	sign := ""
	if abs < 0 {
		abs = -abs
		sign = "-"
	}
	units := abs / 100
	cents := abs % 100
	amountStr := fmt.Sprintf("%s%d.%02d", sign, units, cents)

	type aux struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	return json.Marshal(aux{
		Amount:   amountStr,
		Currency: m.currency,
	})
}

func (m *Money) UnmarshalJSON(data []byte) error {
	type aux struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	var a aux
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	parsed, err := NewMoney(a.Amount, a.Currency)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}
