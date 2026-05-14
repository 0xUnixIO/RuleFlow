package app

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"io"
	"net/netip"

	"github.com/sagernet/sing/common/domain"
	"github.com/sagernet/sing/common/varbin"
)

// SRS 二进制格式常量（sing-box rule-set binary v1）
const (
	srsMagic   = "SRS"
	srsVersion = uint8(1)

	srsRuleTypeDefault = uint8(0)

	srsItemDomain        = uint8(2)
	srsItemDomainKeyword = uint8(3)
	srsItemIPCIDR        = uint8(6)
	srsItemFinal         = uint8(0xFF)

	srsIPSetVersion = uint8(1)
)

// WriteSRS 将规则集编译为 sing-box SRS 二进制格式并写入 w
func WriteSRS(rules []RuleSetRule, w io.Writer) error {
	// 魔数 + 版本
	if _, err := io.WriteString(w, srsMagic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, srsVersion); err != nil {
		return err
	}

	// zlib 压缩层
	zw, err := zlib.NewWriterLevel(w, zlib.BestCompression)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(zw)

	// 规则数量：始终为 1 条 DefaultRule
	if _, err := varbin.WriteUvarint(bw, 1); err != nil {
		return err
	}

	if err := writeSRSDefaultRule(bw, rules); err != nil {
		return err
	}

	if err := bw.Flush(); err != nil {
		return err
	}
	return zw.Close()
}

func writeSRSDefaultRule(w *bufio.Writer, rules []RuleSetRule) error {
	// 按类型分组，过滤空值（防止 domain.NewMatcher 对空字符串 panic）
	var domains, suffixes, keywords, cidrs []string
	for _, r := range rules {
		if r.Value == "" {
			continue
		}
		switch r.Type {
		case "domain":
			domains = append(domains, r.Value)
		case "domain_suffix":
			suffixes = append(suffixes, r.Value)
		case "domain_keyword":
			keywords = append(keywords, r.Value)
		case "ip_cidr":
			cidrs = append(cidrs, r.Value)
		}
	}

	// 规则类型：DefaultRule = 0x00
	if err := w.WriteByte(srsRuleTypeDefault); err != nil {
		return err
	}

	// domain + domain_suffix 合并写入（使用 succinct trie）
	if len(domains) > 0 || len(suffixes) > 0 {
		if err := w.WriteByte(srsItemDomain); err != nil {
			return err
		}
		m := domain.NewMatcher(domains, suffixes, false)
		if err := m.Write(w); err != nil {
			return err
		}
	}

	// domain_keyword：逐个写字符串
	if len(keywords) > 0 {
		if err := writeSRSStringItem(w, srsItemDomainKeyword, keywords); err != nil {
			return err
		}
	}

	// ip_cidr：转为地址范围后写入 IP set
	if len(cidrs) > 0 {
		if err := writeSRSIPCIDR(w, cidrs); err != nil {
			return err
		}
	}

	// 结束标记 + Invert=false
	if err := w.WriteByte(srsItemFinal); err != nil {
		return err
	}
	return w.WriteByte(0x00)
}

// writeSRSStringItem 写入字符串列表项（item_type + count + strings）
func writeSRSStringItem(w *bufio.Writer, itemType uint8, strs []string) error {
	if err := w.WriteByte(itemType); err != nil {
		return err
	}
	if _, err := varbin.WriteUvarint(w, uint64(len(strs))); err != nil {
		return err
	}
	for _, s := range strs {
		if _, err := varbin.WriteUvarint(w, uint64(len(s))); err != nil {
			return err
		}
		if _, err := w.Write([]byte(s)); err != nil {
			return err
		}
	}
	return nil
}

// writeSRSIPCIDR 将 CIDR 列表编码为 sing-box IP set 格式
func writeSRSIPCIDR(w *bufio.Writer, cidrs []string) error {
	type ipRange struct{ from, to []byte }

	ranges := make([]ipRange, 0, len(cidrs))
	for _, s := range cidrs {
		if prefix, err := netip.ParsePrefix(s); err == nil {
			// 规范化 IPv4-mapped IPv6（::ffff:1.2.3.4/120 → 1.2.3.0/24），
			// 确保 from 和 to 的字节长度一致
			prefix = prefix.Masked()
			if prefix.Addr().Is4In6() {
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
			from := prefix.Addr().AsSlice()
			to := srsLastAddr(prefix).AsSlice()
			ranges = append(ranges, ipRange{from, to})
		} else if addr, err := netip.ParseAddr(s); err == nil {
			if addr.Is4In6() {
				addr = addr.Unmap()
			}
			b := addr.AsSlice()
			ranges = append(ranges, ipRange{b, b})
		}
		// 无法解析的条目静默跳过
	}

	if err := w.WriteByte(srsItemIPCIDR); err != nil {
		return err
	}
	// IP set header：版本 + range 数量（uint64 big-endian）
	if err := w.WriteByte(srsIPSetVersion); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(len(ranges))); err != nil {
		return err
	}
	for _, r := range ranges {
		if _, err := varbin.WriteUvarint(w, uint64(len(r.from))); err != nil {
			return err
		}
		if _, err := w.Write(r.from); err != nil {
			return err
		}
		if _, err := varbin.WriteUvarint(w, uint64(len(r.to))); err != nil {
			return err
		}
		if _, err := w.Write(r.to); err != nil {
			return err
		}
	}
	return nil
}

// srsLastAddr 计算前缀的最后一个地址（广播地址）
func srsLastAddr(prefix netip.Prefix) netip.Addr {
	addr := prefix.Masked().Addr()
	// 将 IPv4-mapped IPv6（如 ::ffff:1.2.3.4）规范为纯 IPv4
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	bits := prefix.Bits()
	if addr.Is4() {
		a := addr.As4()
		n := binary.BigEndian.Uint32(a[:])
		// bits=0 时 1<<32 在 uint32 中溢出为 0，用 ^uint32(0)>>bits 替代
		n |= ^uint32(0) >> bits
		var res [4]byte
		binary.BigEndian.PutUint32(res[:], n)
		return netip.AddrFrom4(res)
	}
	// IPv6
	a := addr.As16()
	byteStart := bits / 8
	bitRem := bits % 8
	if bitRem > 0 {
		a[byteStart] |= byte(0xff) >> bitRem
		byteStart++
	}
	for i := byteStart; i < 16; i++ {
		a[i] = 0xff
	}
	return netip.AddrFrom16(a)
}
