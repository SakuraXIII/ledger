package model

import (
	"fmt"
	"math"
	"strconv"
)

type Money int64

func NewMoneyYuan(amount float64) Money {
	return Money(int64(math.Round(amount * 100)))
}

func NewMoney(amount int64) Money {
	return Money(amount)
}

func (m Money) Abs() Money {
	if m < 0 {
		return -m
	}
	return m
}

func (m Money) GetStr() string {
	balanceStr := fmt.Sprintf("%f", float64(m)/100.0)
	return balanceStr
}

func (m Money) ToYuan() float64 {
	return float64(m) / 100.0
}

func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%g", m.ToYuan())), nil
}

func (m *Money) UnmarshalJSON(data []byte) error {
	float, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return fmt.Errorf("反序列化金额失败：%s", err.Error())
	}
	*m = NewMoneyYuan(float)
	return nil
}
