package Format

import "fmt"

type ByteUnit struct {
	Value int
	Units string
}

func (u *ByteUnit) String() string {
	return fmt.Sprintf("%d%s", u.Value, u.Units)
}

type RateUnit struct {
	Value int
	Units string
}

func (u *RateUnit) String() string {
	return fmt.Sprintf("%d%s/s", u.Value, u.Units)
}

const DefaultByteUnit = "B"

func ToByte(value int) *ByteUnit {
	var units = []string{"B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}

	var unit = 0
	for ; value > 1024; value /= 1024 {
		unit++
	}

	return &ByteUnit{
		Value: value,
		Units: units[unit],
	}
}

const DefaultRateUnit = "B/s"

func ToRate(value int) *RateUnit {
	var unit = ToByte(value)

	return &RateUnit{
		Value: unit.Value,
		Units: unit.Units + "/s",
	}
}
