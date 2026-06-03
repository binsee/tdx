package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/injoyai/logs"
	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

// 真实连接通达信服务器，演示 report file(0x06B9) 文件下载能力：
//  1. 下载板块/配置数据总包 zhb.zip，原始字节存 dump/zhb.zip
//  2. 解压 zhb.zip，全部成分文件存 dump/zhb/<file>
//  3. 解析 tdxzs.cfg(板块指数代码 id) → dump/tdxzs.parsed.txt
//  4. 下载 block_*.dat 板块成分，按名称用 tdxzs.cfg 回填板块 id → dump/<block>.withid.txt
func main() {
	dir := "dump"
	logs.PanicErr(os.MkdirAll(filepath.Join(dir, "zhb"), 0755))

	c, err := tdx.DialDefault()
	logs.PanicErr(err)
	defer c.Close()

	// 1. 下载 zhb.zip 原始包
	raw, err := c.GetReportFile(protocol.ReportZHB)
	logs.PanicErr(err)
	logs.PanicErr(os.WriteFile(filepath.Join(dir, protocol.ReportZHB), raw, 0644))
	logs.Infof("%s 原始包 %d 字节\n", protocol.ReportZHB, len(raw))

	// 2. 解压全部成分文件
	files, err := c.GetZHBFiles()
	logs.PanicErr(err)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		logs.PanicErr(os.WriteFile(filepath.Join(dir, "zhb", n), files[n], 0644))
	}
	logs.Infof("解压 %d 个文件 → %s/zhb/\n", len(names), dir)

	// 3. 解析 tdxzs.cfg 板块指数代码(id)
	zs := protocol.ParseTdxZs(files[protocol.FileTdxZs])
	var sb strings.Builder
	fmt.Fprintf(&sb, "tdxzs.cfg 板块指数, 共 %d 个\n\n", len(zs))
	for _, z := range zs {
		fmt.Fprintf(&sb, "id=%-8s 类型=%d/%d 名称=%s\n", z.Code, z.Type, z.SubType, z.Name)
	}
	logs.PanicErr(os.WriteFile(filepath.Join(dir, "tdxzs.parsed.txt"), []byte(sb.String()), 0644))
	logs.Infof("解析 tdxzs.cfg %d 个板块指数 → %s/tdxzs.parsed.txt\n", len(zs), dir)

	// 4. 下载板块成分并回填 id
	for _, bf := range []string{protocol.BlockFileZS, protocol.BlockFileGN, protocol.BlockFileFG} {
		blocks, err := c.GetBlockData(bf)
		if err != nil {
			logs.Err(bf, err)
			continue
		}
		matched := protocol.FillBlockIndex(blocks, zs)

		var b strings.Builder
		fmt.Fprintf(&b, "%s 共 %d 个板块, 命中 id %d 个\n\n", bf, len(blocks), matched)
		for _, blk := range blocks {
			fmt.Fprintf(&b, "id=%-8s 类型=%d 成分=%d 名称=%s\n  %s\n", blk.Index, blk.Type, len(blk.Codes), blk.Name, strings.Join(blk.Codes, " "))
		}
		logs.PanicErr(os.WriteFile(filepath.Join(dir, bf+".withid.txt"), []byte(b.String()), 0644))
		logs.Infof("%-14s %d 板块, id 命中 %d → %s/%s.withid.txt\n", bf, len(blocks), matched, dir, bf)
	}
}
