package web

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func stellarDisplayDate(d *StellarRenderData, value any) string {
	var date time.Time
	switch v := value.(type) {
	case time.Time:
		date = v
	case *time.Time:
		if v != nil {
			date = *v
		}
	default:
		s := stellarText(value)
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, s); err == nil {
				date = t
				break
			}
		}
	}
	if date.IsZero() {
		return ""
	}
	format := stellarText(stellarFirst(d.Stellar.Data["dateFormat"], "YYYY-MM-DD"))
	return stellarMomentFormat(date, format, d.Stellar.Language)
}

// stellarMomentFormat ports the Moment display tokens accepted by Hexo's
// date_format, including bracketed literals, localized forms and ordinals.
// A date retains its source offset; host timezone conversion precedes rendering.
func stellarMomentFormat(date time.Time, format, locale string) string {
	english := strings.HasPrefix(strings.ToLower(locale), "en")
	traditional := strings.EqualFold(locale, "zh-TW") || strings.EqualFold(locale, "zh-HK")
	months := []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	weekdays := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	month, weekday := months[int(date.Month())-1], weekdays[int(date.Weekday())]
	shortMonth, shortDay := month[:3], weekday[:3]
	dayMin := weekday[:2]
	amPM := "AM"
	if date.Hour() >= 12 {
		amPM = "PM"
	}
	ordinal := func(n int, kind string) string {
		if !english {
			suffix := "日"
			if kind == "Q" {
				return strconv.Itoa(n)
			}
			if kind == "M" {
				suffix = "月"
			} else if kind == "w" {
				suffix = "周"
				if traditional {
					suffix = "週"
				}
			}
			return strconv.Itoa(n) + suffix
		}
		suffix := "th"
		if n%100 < 11 || n%100 > 13 {
			switch n % 10 {
			case 1:
				suffix = "st"
			case 2:
				suffix = "nd"
			case 3:
				suffix = "rd"
			}
		}
		return strconv.Itoa(n) + suffix
	}
	localized := map[string]string{"L": "MM/DD/YYYY", "LL": "MMMM D, YYYY", "LLL": "MMMM D, YYYY h:mm A", "LLLL": "dddd, MMMM D, YYYY h:mm A", "LT": "h:mm A", "LTS": "h:mm:ss A", "l": "M/D/YYYY", "ll": "MMM D, YYYY", "lll": "MMM D, YYYY h:mm A", "llll": "ddd, MMM D, YYYY h:mm A"}
	if !english {
		month = []string{"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"}[date.Month()-1]
		shortMonth = fmt.Sprintf("%d月", date.Month())
		names := []string{"日", "一", "二", "三", "四", "五", "六"}
		weekday = "星期" + names[date.Weekday()]
		shortDay = "周" + names[date.Weekday()]
		if traditional {
			shortDay = "週" + names[date.Weekday()]
		}
		dayMin = names[date.Weekday()]
		hour := date.Hour()*100 + date.Minute()
		switch {
		case hour < 600:
			amPM = "凌晨"
		case hour < 900:
			amPM = "早上"
		case hour < 1130:
			amPM = "上午"
		case hour < 1230:
			amPM = "中午"
		case hour < 1800:
			amPM = "下午"
		default:
			amPM = "晚上"
		}
		localized = map[string]string{"L": "YYYY/MM/DD", "LL": "YYYY年M月D日", "LLL": "YYYY年M月D日Ah点mm分", "LLLL": "YYYY年M月D日ddddAh点mm分", "LT": "HH:mm", "LTS": "HH:mm:ss", "l": "YYYY/M/D", "ll": "YYYY年M月D日", "lll": "YYYY年M月D日 HH:mm", "llll": "YYYY年M月D日dddd HH:mm"}
	}
	if traditional {
		localized["LLL"] = "YYYY年M月D日 HH:mm"
		localized["LLLL"] = "YYYY年M月D日dddd HH:mm"
	}
	hour12 := date.Hour() % 12
	if hour12 == 0 {
		hour12 = 12
	}
	hour24 := date.Hour()
	if hour24 == 0 {
		hour24 = 24
	}
	isoYear, isoWeek := date.ISOWeek()
	week, weekYear := stellarLocaleWeek(date, english || traditional)
	offset := date.Format("-07:00")
	zone := date.Format("-0700")
	isoDay := int(date.Weekday())
	if isoDay == 0 {
		isoDay = 7
	}
	localDay := int(date.Weekday())
	if !english && !traditional {
		localDay = (localDay + 6) % 7
	}
	quarter := (int(date.Month())-1)/3 + 1
	values := map[string]string{"YYYY": fmt.Sprintf("%04d", date.Year()), "YY": fmt.Sprintf("%02d", date.Year()%100), "Y": strconv.Itoa(date.Year()), "Q": strconv.Itoa(quarter), "Qo": ordinal(quarter, "Q"), "MMMM": month, "MMM": shortMonth, "MM": fmt.Sprintf("%02d", date.Month()), "M": strconv.Itoa(int(date.Month())), "Mo": ordinal(int(date.Month()), "M"), "DDDD": fmt.Sprintf("%03d", date.YearDay()), "DDD": strconv.Itoa(date.YearDay()), "DDDo": ordinal(date.YearDay(), "D"), "DD": fmt.Sprintf("%02d", date.Day()), "D": strconv.Itoa(date.Day()), "Do": ordinal(date.Day(), "D"), "dddd": weekday, "ddd": shortDay, "dd": dayMin, "d": strconv.Itoa(int(date.Weekday())), "do": ordinal(int(date.Weekday()), "d"), "e": strconv.Itoa(localDay), "E": strconv.Itoa(isoDay), "HH": fmt.Sprintf("%02d", date.Hour()), "H": strconv.Itoa(date.Hour()), "hh": fmt.Sprintf("%02d", hour12), "h": strconv.Itoa(hour12), "kk": fmt.Sprintf("%02d", hour24), "k": strconv.Itoa(hour24), "mm": fmt.Sprintf("%02d", date.Minute()), "m": strconv.Itoa(date.Minute()), "ss": fmt.Sprintf("%02d", date.Second()), "s": strconv.Itoa(date.Second()), "A": amPM, "a": strings.ToLower(amPM), "ZZ": zone, "Z": offset, "WW": fmt.Sprintf("%02d", isoWeek), "W": strconv.Itoa(isoWeek), "Wo": ordinal(isoWeek, "w"), "ww": fmt.Sprintf("%02d", week), "w": strconv.Itoa(week), "wo": ordinal(week, "w"), "GGGG": strconv.Itoa(isoYear), "GG": fmt.Sprintf("%02d", isoYear%100), "gggg": strconv.Itoa(weekYear), "gg": fmt.Sprintf("%02d", weekYear%100), "X": strconv.FormatInt(date.Unix(), 10), "x": strconv.FormatInt(date.UnixMilli(), 10), "z": "UTC", "zz": "Coordinated Universal Time"}
	for i := 1; i <= 9; i++ {
		key := strings.Repeat("S", i)
		value := fmt.Sprintf("%09d", date.Nanosecond()/1e6*1e6)
		if i < 3 {
			value = value[:i]
		} else if i <= 9 {
			value = value[:i]
		}
		values[key] = value
	}
	tokens := []string{"LLLL", "llll", "LTS", "LLL", "lll", "LL", "ll", "LT", "L", "l", "DDDo", "YYYY", "MMMM", "DDDD", "dddd", "GGGG", "gggg", "SSSSSSSSS", "SSSSSSSS", "SSSSSSS", "SSSSSS", "SSSSS", "SSSS", "SSS", "MMM", "DDD", "ddd", "SS", "YY", "MM", "Mo", "DD", "Do", "dd", "do", "HH", "hh", "kk", "mm", "ss", "ZZ", "WW", "Wo", "ww", "wo", "GG", "gg", "Qo", "Y", "Q", "M", "D", "d", "e", "E", "H", "h", "k", "m", "s", "S", "A", "a", "Z", "W", "w", "X", "x", "zz", "z"}
	var out strings.Builder
	for i := 0; i < len(format); {
		if format[i] == '[' {
			if end := strings.IndexByte(format[i+1:], ']'); end >= 0 {
				out.WriteString(format[i+1 : i+1+end])
				i += end + 2
				continue
			}
		}
		if format[i] == '\\' && i+1 < len(format) {
			out.WriteByte(format[i+1])
			i += 2
			continue
		}
		matched := false
		for _, token := range tokens {
			if strings.HasPrefix(format[i:], token) {
				if expanded, ok := localized[token]; ok {
					out.WriteString(stellarMomentFormat(date, expanded, locale))
				} else {
					out.WriteString(values[token])
				}
				i += len(token)
				matched = true
				break
			}
		}
		if !matched {
			out.WriteByte(format[i])
			i++
		}
	}
	return out.String()
}

func stellarLocaleWeek(date time.Time, sundayFirst bool) (int, int) {
	if !sundayFirst {
		year, week := date.ISOWeek()
		return week, year
	}
	offset := func(year int) int { jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC); return -int(jan1.Weekday()) }
	weeks := func(year int) int {
		days := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24
		return int((days - float64(offset(year)) + float64(offset(year+1))) / 7)
	}
	year := date.Year()
	week := int(math.Floor(float64(date.YearDay()-offset(year)-1)/7)) + 1
	if week < 1 {
		year--
		week += weeks(year)
	} else if week > weeks(year) {
		week -= weeks(year)
		year++
	}
	return week, year
}
