package app

import (
	"strings"
	"unicode"
)

// countryRules 提供国家关键词；最终按名称中最靠左的合法命中决定 emoji。
var countryRules = []struct {
	emoji    string
	keywords []string
}{
	{"🇭🇰", []string{"香港", "hk", "hong kong", "hongkong"}},
	{"🌏", []string{"台湾", "台灣", "tw", "taiwan"}},
	{"🇯🇵", []string{"日本", "jp", "japan", "tokyo", "osaka", "东京", "大阪"}},
	{"🇰🇷", []string{"韩国", "韓國", "kr", "korea", "seoul", "首尔"}},
	{"🇸🇬", []string{"新加坡", "狮城", "sg", "singapore"}},
	{"🇺🇸", []string{"美国", "美國", "us", "united states", "america", "los angeles", "new york", "seattle", "silicon valley", "洛杉矶", "纽约", "西雅图"}},
	{"🇬🇧", []string{"英国", "英國", "uk", "united kingdom", "britain", "london", "伦敦"}},
	{"🇩🇪", []string{"德国", "德國", "de", "germany", "berlin", "frankfurt", "柏林", "法兰克福"}},
	{"🇫🇷", []string{"法国", "法國", "fr", "france", "paris", "巴黎"}},
	{"🇳🇱", []string{"荷兰", "荷蘭", "nl", "netherlands", "amsterdam"}},
	{"🇨🇦", []string{"加拿大", "ca", "canada", "toronto", "vancouver"}},
	{"🇦🇺", []string{"澳大利亚", "澳洲", "悉尼", "墨尔本", "au", "australia", "sydney", "melbourne"}},
	{"🇷🇺", []string{"俄罗斯", "俄羅斯", "ru", "russia", "moscow", "莫斯科"}},
	{"🇮🇳", []string{"印度", "in", "india", "mumbai", "chennai"}},
	{"🇧🇷", []string{"巴西", "br", "brazil"}},
	{"🇦🇷", []string{"阿根廷", "ar", "argentina"}},
	{"🇲🇾", []string{"马来西亚", "馬來西亞", "my", "malaysia", "kuala lumpur"}},
	{"🇵🇭", []string{"菲律宾", "菲律賓", "ph", "philippines", "manila"}},
	{"🇹🇭", []string{"泰国", "泰國", "th", "thailand", "bangkok"}},
	{"🇻🇳", []string{"越南", "vn", "vietnam"}},
	{"🇮🇩", []string{"印度尼西亚", "印尼", "id", "indonesia", "jakarta"}},
	{"🇹🇷", []string{"土耳其", "tr", "turkey", "istanbul"}},
	{"🇦🇪", []string{"阿联酋", "阿聯酋", "ae", "uae", "dubai", "迪拜"}},
	{"🇸🇦", []string{"沙特", "sa", "saudi"}},
	{"🇮🇱", []string{"以色列", "il", "israel"}},
	{"🇵🇱", []string{"波兰", "波蘭", "pl", "poland"}},
	{"🇸🇪", []string{"瑞典", "se", "sweden"}},
	{"🇨🇭", []string{"瑞士", "ch", "switzerland"}},
	{"🇳🇴", []string{"挪威", "no", "norway"}},
	{"🇫🇮", []string{"芬兰", "芬蘭", "fi", "finland"}},
	{"🇩🇰", []string{"丹麦", "丹麥", "dk", "denmark"}},
	{"🇪🇸", []string{"西班牙", "es", "spain", "madrid"}},
	{"🇮🇹", []string{"意大利", "義大利", "it", "italy", "milan", "rome"}},
	{"🇵🇹", []string{"葡萄牙", "pt", "portugal"}},
	{"🇦🇹", []string{"奥地利", "奧地利", "at", "austria"}},
	{"🇧🇪", []string{"比利时", "比利時", "be", "belgium"}},
	{"🇨🇿", []string{"捷克", "cz", "czech"}},
	{"🇭🇺", []string{"匈牙利", "hu", "hungary"}},
	{"🇷🇴", []string{"罗马尼亚", "羅馬尼亞", "ro", "romania"}},
	{"🇺🇦", []string{"乌克兰", "烏克蘭", "ua", "ukraine"}},
	{"🇿🇦", []string{"南非", "za", "south africa"}},
	{"🇲🇽", []string{"墨西哥", "mx", "mexico"}},
	{"🇨🇱", []string{"智利", "cl", "chile"}},
}

// addCountryEmoji 若节点名尚未以 emoji 开头，则按关键词匹配国家并在前面加上 emoji
func addCountryEmoji(name string) string {
	if startsWithEmoji(name) {
		return name
	}
	lower := strings.ToLower(name)
	bestIdx := -1
	bestEmoji := ""
	for _, rule := range countryRules {
		for _, kw := range rule.keywords {
			// 用词边界方式匹配：关键词两侧不是字母/数字即视为命中
			if idx, ok := wordIndex(lower, kw); ok && (bestIdx < 0 || idx < bestIdx) {
				bestIdx = idx
				bestEmoji = rule.emoji
			}
		}
	}
	if bestEmoji != "" {
		return bestEmoji + " " + name
	}
	return name
}

// startsWithEmoji 判断字符串首字符是否为 emoji
func startsWithEmoji(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		return isEmojiRune(r)
	}
	return false
}

func isEmojiRune(r rune) bool {
	return r > 0x2000 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-'
}

// wordIndex 返回 text 中最靠左的合法关键词位置。
// word 两侧不能紧跟字母（防止 "ru" 命中 "russia" 内部），但允许数字和符号。
func wordIndex(text, word string) (int, bool) {
	requireBoundary := asciiWordBoundaryRequired(word)
	searchFrom := 0
	for searchFrom <= len(text)-len(word) {
		rel := strings.Index(text[searchFrom:], word)
		if rel < 0 {
			return -1, false
		}
		idx := searchFrom + rel
		if requireBoundary && idx > 0 {
			prev := rune(text[idx-1])
			if unicode.IsLetter(prev) {
				searchFrom = idx + len(word)
				continue
			}
		}
		end := idx + len(word)
		if requireBoundary && end < len(text) {
			next := rune(text[end])
			if unicode.IsLetter(next) {
				searchFrom = idx + len(word)
				continue
			}
		}
		return idx, true
	}
	return -1, false
}

func asciiWordBoundaryRequired(word string) bool {
	for _, r := range word {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return false
		}
	}
	return word != ""
}
