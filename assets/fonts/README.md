# 字体源文件

界面用两族字体，源文件都**不进仓库**（合计约 62 MB），只有子集化产物进仓库。

## 放什么

```
assets/fonts/
├── src/                                 ← 手动下载，被 .gitignore 忽略
│   ├── KingHwaOldSong.ttf               36.6 MB  v2.002
│   └── LXGWWenKaiScreen.ttf             26.1 MB  v1.530
├── LICENSE-KingHwaOldSong.txt           作者声明（逐字转录）
├── LICENSE-LXGWWenKaiScreen.txt         SIL OFL 1.1 全文
└── README.md                            本文件
```

## 怎么下载

| 字体 | 地址 |
|---|---|
| 京华老宋体 v2.002 | https://raw.githubusercontent.com/cntrump/imitation_typeface_fonts/main/%E4%BA%AC%E8%8F%AF%E8%80%81%E5%AE%8B%E4%BD%93v2.002.ttf |
| 霞鹜文楷 Screen v1.530 | https://github.com/lxgw/LxgwWenKai-Screen/releases/download/v1.530/LXGWWenKaiScreen.ttf |

重命名为 `KingHwaOldSong.ttf` / `LXGWWenKaiScreen.ttf` 放进 `src/`。

## 怎么生成子集

```
python scripts/subset-fonts.py
```

产物写到 `frontend/public/fonts/`：

| 文件 | 字符数 | 体积 |
|---|---|---|
| `kinghwa-oldsong-subset.woff2` | 377 | 293 KB |
| `lxgw-wenkai-screen-subset.woff2` | 3899 | 1003 KB |

合计 1.27 MB，在 DESIGN §3.5 的 2.6 MB 预算内。

### 两族为什么策略不同

**京华老宋体只收界面源码里出现过的字**（`frontend/src` 的 .vue/.ts/.html）+ 内置城市表的城市名 + ASCII 与标点。它是展示字，只出现在字标、标题、大号读数、首次启动那句话里，用户不可能让它渲染一段自由文本。

**霞鹜文楷收 GB2312 一级字库（3755 字）**。它是正文字，要渲染用户能看到的每一个标签、城市名、错误文案，还会渲染未来新增的文案。少收一个字就是一块豆腐，多收 3000 字只多几百 KB。

城市名必须单独补进展示字子集：首次启动那句话是「天色按**杭州**的日落工作」，城市名用京华老宋体渲染。

## 子集化注意

- `--layout-features=*` 保留全部 OpenType 特性。京华老宋体带 `fwid`/`pwid`/`zero`（无 `tnum`），霞鹜文楷带 `calt`/`chws`/`halt`，丢掉会改变排版。
- 两族都只有 **Regular 一个字重**（`usWeightClass 400`）。层级只靠字号与颜色，**禁止伪粗体**（`-webkit-text-stroke`、`text-shadow` 模拟）。
- 京华老宋体的数字不等宽（0.507–0.595 em），`tnum` 也不存在，所以大号读数用逐位定宽 span 实现等宽（DESIGN §3.4）。
