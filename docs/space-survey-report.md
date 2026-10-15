# 飞牛 NAS 系统空间占用勘察报告

- 设备：fnOS OECT，aarch64，`192.168.0.2:8666`
- 勘察方式：`trim-cli --profile fnoslogman --allow-insecure-ws`
- 勘察日期：2026-10-15（规则修正后于同日复核）
- 说明：报告中的体积为 `file calc` 实测值，未做估算。

## 一、总体空间

| 位置 | 设备/文件系统 | 总容量 | 已用 | 可用 | 使用率 |
| --- | --- | --- | --- | --- | --- |
| 系统分区 | eMMC（`/dev/mmcblk0p2`，btrfs），挂载点为 `/` | 7,261,388,800 B（6.76 GiB） | 4,363,968,512 B（4.06 GiB） | 2,897,420,288 B（2.70 GiB） | **60.1%** |
| boot 分区 | eMMC（`/dev/mmcblk0p1`，ext4） | 235,125,760 B | 79,972,352 B（76.3 MB） | 155,153,408 B（148.0 MB） | 34.0% |
| /vol1 | btrfs basic，PLEXTOR PX-256M8VC 256 GB | 254,605,475,840 B | ~54.2 GB | 200,383,606,784 B（186.6 GiB） | ~21% |
| 外接盘 | WD10SDRW 1 TB USB（sdb1） | 1,000,169,517,056 B | — | 44,331,905,024 B | — |

数据来源：`storage overview`（各卷 `fssize`/`frsize`）与
`file sys-part-info`（`fssize` 7,261,388,800 / `frsize` 2,897,420,288，末次读取）。

> **两个 fnOS 接口对系统分区的口径不一致——写代码时不能只信一个。**
> 同一时刻实测：
>
> | 接口 | fssize | frsize | 由此算出的「已用」 |
> | --- | --- | --- | --- |
> | `file sys-part-info` | 7,261,388,800 | 2,897,420,288 | 4,363,968,512 |
> | `storage overview` | 6,978,158,592 | 2,614,190,080 | 4,363,968,512 |
>
> 两者**「已用」完全一致**，但 `fssize`/`frsize` 各差一个**恒定偏移 283,230,208 B
> （270.1 MB）**——这是 btrfs 保留块（reserved）计入与否的差别。后果是可用空间会
> 报出 2.70 GiB 与 2.43 GiB 两个值、使用率会报出 60.1% 与 62.5% 两个值。
> 因此**任何硬编码的系统分区容量都是错的**；本应用改为运行时调 `diskUsageOf`
> 现读（`scanVolumeUsage`），不做静态假设。

**关键结论：真正紧张的是 6.76 GiB 的系统分区（可用约 2.43–2.70 GiB，取决于口径），
而不是 /vol1（还剩 186 GiB）。**
但 `trim-cli` 的文件命令只接受 `/vol{v}/...` 规范路径，`/var`、`/tmp`、`/usr/trim`、
`/boot`、`/usr/share/doc`、`/var/log/journal`、`/usr/lib/linux-image-*`
**一律返回 `Unsupported path`**（本轮已逐条复测），所以系统分区只能由运行在
NAS 上、以 root 身份运行的应用自身来扫描——这正是把清理功能做进本应用的原因。

> **可用空间已在变化，且已被清理过一轮**：本会话实测时间线如下
>
> | 时点 | `frsize`（sys-part-info） |
> | --- | --- |
> | 会话早期 | 2,629,140,480 B |
> | 中期 | 2,898,501,632 B |
> | 末期（连续 3 次采样稳定） | 2,897,420,288 B |
>
> 净增约 268 MB。外部同期观测也独立印证：`/boot` 已用从 124 MB 降到 76.3 MB
> （**本轮我用自己的 `storage overview` 实测确认为 79,972,352 B = 76.3 MB，吻合**）。
> 结论：系统分区的数字必须现读现用，任何静态快照都会迅速过期。

> **设备型号更正**：系统盘是 **eMMC**（`/dev/mmcblk0`，`storage overview` 的
> `block` 列表里 `mmcblk0` 无 `modelName`，符合 eMMC 特征），
> 而 `file sys-part-info` 的 `type` 字段报 `"SSD"`——这是把 eMMC 归入 SSD 档的
> 粗粒度标签，两者不矛盾，但报告应以 `/dev/mmcblk0p2` 为准。

> **btrfs 未分配空间偏少**：底层 Data 5.97 GiB / 已用 4.04 GiB，
> **未分配仅剩 281 MiB**。btrfs 在未分配空间耗尽时，即使还有可用块也可能
> 出现写失败，因此这台设备确实处于需要关注的状态。
>
> **`du` 与 `df` 不一致是正常的**：`du` 统计 `/usr` 为 5.5 G，`df` 只有 4.4 G。
> 该分区未启用压缩、硬链接仅 13 个，差异来自 **btrfs extent 共享**（同一 extent
> 被多个文件引用时 `du` 会重复计数）。**以 `df` 为准**——这一点在写清理规则时
> 很重要：不能拿 `du` 的数值去预估能回收多少空间，否则会系统性高估。

### 一之二、系统分区（`/`）上的可删项——**本应用当前完全未覆盖**

系统分区的占用内容无法经 `trim-cli` 枚举（只接受 `/volN`），以下条目为**外部提供、
未经本会话独立复测**，仅作规划参考。

**第二轮分析（22:26）已确认第一轮清理真实发生过**，因此 A 档从 546 MB 降为约 326 MB：

| 档 | 项目 | 路径 | 大小 | 状态 / 风险判断 |
| --- | --- | --- | --- | --- |
| A | ~~陈旧内核 c1090（模块+头文件）~~ | `/usr/lib/modules/6.18.18.c1090-trim`、`/usr/src/linux-headers-…c1090-trim` | ~~253 MB~~ 已消失 | **第一轮已清理** |
| A | ~~陈旧内核引导文件~~ | `/boot/vmlinuz-6.18.18.c1090-trim` 等 | ~~47 MB~~ 已消失 | **第一轮已清理** |
| A | **孤儿内核 DTB 目录（第一轮遗漏）** | `/usr/lib/linux-image-6.18.18.{c1090,c994}-trim` | 36 MB | 手工 `rm` 而非 `apt purge` 留下的尾巴，需用 `apt purge` 收尾 |
| A | apt 二进制索引缓存 | `/var/cache/apt/pkgcache.bin`、`srcpkgcache.bin` | 77 MB | 可再生（`apt-get update` 重建） |
| A | apt 软件源索引 | `/var/lib/apt/lists/*` | 85 MB | 可再生 |
| A | 系统日志历史 | `/var/log/{syslog.1,syslog,kern.log,auth.log}` | ~82 MB | 需**截断**而非删除（有进程持句柄）；`syslog.1` 单项 50.3 MB |
| A | 服务日志 | samba、cloud_storage_dav、trim_open_gateway、trim_app_center | ~23 MB | 同上 |
| A | sysstat 历史 | `/var/log/sysstat/sar*` | 9.1 MB | 历史采样，可弃 |
| A | 已删除但仍被占用 | pid 4308(deepseek.harness) 14.0M、pid 4664(trim.music) 0.57M | ~15 MB | 需截断 `/proc/<pid>/fd/*` 才真正释放 |
| A | dpkg 空壳记录 | 6 个 header 包 + 2 个 image 包（`hi`/`ii` 态） | ≈ 0 MB | 不回收空间，但包数据库与实际文件已不符，应整理 |
| A | 杂项残留 | `drivedb.h.old`、`*.dpkg-dist`、`*.old`、`sm.bak`、`/boot/vmlinuz.old`（悬空链接） | < 1 MB | 逐项确认后删 |
| B | ARM 虚拟机 UEFI 固件 | `/usr/share/AAVMF`（qemu-efi-aarch64/arm） | 386 MB | `virsh list --all` 为空、`/var/lib/libvirt/images` 为空，但将来建 ARM 虚拟机就需要 |
| B | 无线网卡固件 | `/usr/lib/firmware`（mediatek 25M、rtw89 19M、aic8800 12M…） | 74 MB | 已核实无任何无线接口（`/sys/class/net/` 仅 `end0` + docker 网桥）；但插 USB 无线网卡就需要，属功能性组件 |
| B | 其他语种 locale | `/usr/share/locale`（保留 zh_CN/en，其余 30+ 语种） | ~180 MB 可省 | 建议用 `localepurge` 而非手工 `rm` |
| B | 文档 | `/usr/share/doc` + `/usr/share/man` | 152 MB | 纯文档；手工 `rm` 会让 `dpkg -V` 报缺失 |
| B | Go 工具链 | `/usr/local/go` | 261 MB | **已排除假警报**：引用它的是 Go 编译期嵌入的 build-info 字符串，非运行时依赖 |
| B | GCC 工具链 | `/usr/lib/gcc` (102M) + gcc-12 (57M) | ~160 MB | 仅本地编译时需要 |
| B | 语音识别模型 | `/usr/share/pocketsphinx` | 36 MB | 被 `libpocketsphinx3` 依赖，删会连带卸载依赖链 |
| B | PROJ 坐标系数据 | `/usr/share/proj` | 22 MB | 被 `libproj25` 依赖，可能被相册定位使用 |
| C | **切勿删除** | c1145 内核三件套（`/usr/lib/modules`、`/usr/lib/linux-image`、`/boot`）212M+187M+19M、c1145 头文件 67M、`/usr/trim` 822M、`/usr/lib/aarch64-linux-gnu` 1.1G、`libLLVM-14/15`、`docker-ce`、`/var/lib/{dpkg,postgresql}` | — | 运行内核 / 系统命脉 / 被反向依赖 |

**对本项目的直接影响：A 档这 326 MB 目前一条规则都命中不了。**
`spaceRules` 的全部 15 条规则都以 `/volN` 为根（`@appdata/*`、`@apptemp/*`、
`thumb/*`、`docker/containers/*`、`appcenter-downloads/*`、`*/AppData/*/dep_cache`），
`system-temp` 一类也仅覆盖 `/tmp`、`/var/tmp`。系统分区在扫描结果里只作为
**容量条**出现（`scanVolumeUsage` 把 `/` 追加进挂载点列表），
**没有任何条目级覆盖**。若要覆盖 A 档，需要新增一类"系统分区包管理与内核残留"
规则，且必须比现有规则更保守：
- 内核包必须**先确认非当前运行内核、且无引导入口**才能删（`uname -r` +
  `extlinux.conf` 双重校验），否则删错内核会导致设备无法启动；
- **必须走 `apt purge` 而不是 `rm`**——第一轮手工 `rm` 就把 dpkg 数据库留在了
  `hi`（半安装）态、并漏掉 36 MB 的 `/usr/lib/linux-image/*` DTB 目录，
  这是"绕过包管理器"的直接代价；
- `/var/lib/apt/lists`、`/var/cache/apt` 属 `apt` 管理的目录，绕过包管理器直删
  会让 `apt` 状态与磁盘不一致，宜走 `apt-get clean` 语义；
- `/var/log/*` 必须**截断**（与 `docker/*-json.log` 同一策略），因为 syslog 等
  由 rsyslog 持句柄，unlink 不释放空间且丢日志；
- `/usr/share/{locale,doc,man}`、`/usr/local/go`、`/usr/lib/firmware` 属 B 档，
  风险是"功能还在但依赖被删"，不应默认勾选，只宜提示。

#### 已修复：内核清理功能不闭环（A 档第 1/8 项）

用户提出一个关键质疑——**"内核不是项目里已有功能么，怎么还残留 dpkg 空壳记录？"**
核对后确认：项目里确实已有一整套内核清理功能，但它**只删文件、不经包管理器**：

| 位置 | 实现 | 删什么 |
| --- | --- | --- |
| `routes/kernel.go` | `GET /kernel/versions` | 列出内核 + 未使用大小 |
| | `POST /kernel/versions/cleanup` | 清理全部非当前内核 |
| | `POST /kernel/versions/:version/remove` | 删除指定内核 |
| `removeKernelVersion` | `os.Remove` / `os.RemoveAll` | `/boot/*-<ver>*`、`/lib/modules/<ver>`、`/usr/src/linux-headers-<ver>*` |

全程没有 `dpkg`/`apt` 参与，而 dpkg 包状态存在 `/var/lib/dpkg/status`——删文件不会动它。
所以那些空壳记录**不是"清理没做干净"，而是"清理机制本身不碰包数据库"**。

同时发现一个真实的覆盖面缺口：`getInstalledKernels` 只扫 `/boot`、`/lib/modules`、
`/usr/src` 三处，**不扫 `/usr/lib/linux-image-*`**。而"某内核是否已安装"的判定
也依赖这三处，因此内核主体一旦被删，那 36 MB 的 DTB 目录就**从列表里彻底消失、
再也不会被这个功能发现**——成为永久盲区（c994 的 DTB 仍在正是这个原因）。

**已做的修改**（只动 `routes/kernel.go` 与前端展示，不新增清理分类）：

1. `getInstalledKernels` 增加扫描 `/usr/lib/linux-image-*`，孤儿 DTB 重新可见；
2. `calcImageSize` 统计 DTB 大小并计入 `TotalSize`（"清理旧内核"因此不再少报）；
3. `removeKernelVersion` 一并清理 `/usr/lib/linux-image-<version>/`；
4. 新增 `dpkgKernelPackagesFn` 注入缝隙 + `dpkg-query` 查询，把"仍记录着该内核的
   包名"作为 `dpkgPackages` 返回；
5. `cleanup` / `remove` 响应新增 `dpkgLeftovers` 与 `dpkgHint`，**显式说明
   "文件已删除，但 dpkg 包记录仍在（此功能只删文件、不经包管理器）"并给出
   可直接粘贴的 `apt purge -y …` 命令**——把原本静默的边界变成可见事实；
6. 前端新增"设备树"列与 dpkg 残留提示块，并修掉一处既有缺陷：
   `removeVersion` 曾用原始字节数 `.toLocaleString() + ' B'` 拼字符串，
   会显示成 `1,234,567 B` 而非格式化体积。

**安全性**：`calcImageSize`/删除路径均以 `os.Lstat` + `IsDir` 守卫，路径不存在即
静默跳过。DTB 路径约定来自设备侧审计、**本机无法复测**（`trim-cli` 拒绝 `/usr`），
因此代码按"可选附加项"处理：**路径猜错只会退化成修改前的行为，不会误删或误报**。
`dpkg-query` 一律以 argv 传参、从不经 shell（版本号来自路径，若经 shell 则可注入）。

**验证**：新增 `kernel_test.go`（7 个顶层 / 21 个子测试）与
`utils/kernelResidue.test.ts`（13 个），并做变异测试确认测试有效：

| 变异 | 结果 |
| --- | --- |
| 去掉 `isValidKernelVersion` 校验 | CAUGHT |
| 去掉 `linux-image-` 前缀检查 | CAUGHT |
| `aptPurgeHint` 空列表也返回命令 | CAUGHT |
| `dpkgResidueHint` 无残留也提示 | CAUGHT |
| `dpkg-query` 改用 shell 执行 | CAUGHT |
| `removeKernelVersion` 不再删 DTB 目录 | CAUGHT |
| 前端：`undefined` 与 `[]` 混同（真值判断） | CAUGHT |
| 前端：不校验是否数组（`null` 会抛错） | CAUGHT |
| 前端：不剔除空串 | CAUGHT |
| 前端：不去重 | CAUGHT |
| 前端：无残留也生成 purge 命令 | CAUGHT |

> 值得一提的过程教训：第一版前端测试里有一条"MISSED"——测试名声称验证
> "`undefined` 不得被当成无残留"，但两种实现在这个函数的返回值上其实**不可区分**，
> 那句断言在任何实现下都成立。已改为按真实可观测行为断言，并补上能真正区分
> 实现的用例（`dpkgPackages` 为 `null` / 非数组时不得抛错）。**变异测试的价值
> 正在于此：它揭穿了"测试通过但没测到东西"。**

> **一条被否定的收益项，值得记下来**：`journalctl --disk-usage` 报 75.3 MB，
> 但 `/var/log/journal` **不存在**，journal 实际位于 `/run/log/journal`（tmpfs）。
> 清理它**不会释放系统盘空间**。这类"看起来在系统盘、其实在 tmpfs"的路径
> 是本设备上最容易误判的一类，规则设计时必须以实际挂载点为准，
> 不能按路径名推断。


> 关于 `/tmp`、`/var/tmp` 是否位于系统分区：`trim-cli` 的 `file`/`storage`
> 子命令只接受 `/volN`，无法读取它们的挂载信息，因此**不做静态断言**。
> 应用改为运行时比较设备号（`systemTempOnSystemDisk`）后按实测结果显示文案：
> 同分区才提示"/tmp 与 /var/tmp 位于此处"，不同分区提示两者不在该分区，
> 读不到设备号则完全不提示——因为 `/tmp` 可能是 tmpfs，那时清理释放的是内存
> 而不是该分区空间。

### 一之三、系统分区清理规则已实施

A 档中可自动化的部分（apt 缓存、日志、sysstat、零散残留）已实现为
`services/spaceclean_system.go`，新增 4 个分类接入既有的 `SpaceCleanPanel`：

| 分类 ID | 名称 | 策略 | 内容 |
| --- | --- | --- | --- |
| `system-pkg-cache` | 系统包缓存（系统分区） | delete | `/var/cache/apt/{pkgcache,srcpkgcache}.bin` |
| `system-logs` | 系统日志（系统分区） | **truncate / delete** | 正在写入的 syslog/kern.log/auth.log 等**截断**；已轮转归档文件（`*.1`、`*.gz`）**删除**；各服务日志目录；`/var/log/sysstat/*` |
| `system-residue` | 系统零散残留 | delete | `drivedb.h.old`、`*.dpkg-dist`、`cdroms.list~`、`sm.bak` |
| `system-pkg-mgr` | 需包管理器处理（仅提示） | 不清理 | `/var/lib/apt/lists` —— 只给出建议命令，本应用不代执行 |

**四层安全设计（比 /volN 规则表更保守）：**

1. **绝对路径规则表**：系统路径不在任何卷下，不能复用"相对卷根"的展开逻辑，
   因此单独一张 `systemPathRules` + 单独的 `matchSystemRule`。
2. **兜底拒绝名单 `systemPathIsForbidden`**：即使规则表被误改，`/boot`、
   `/usr/lib/modules`、`/usr/src`、`/usr/lib/linux-image-`、`/etc/passwd`、
   `/var/lib/dpkg`、`/usr/trim`、`/usr/local/apps` 等也永远不被接受。
   这是纵深防御的最后一层——系统分区删错的后果（设备无法启动）远比其他分区严重。
   匹配按**路径段**而非裸字符串前缀，拒绝 `/boot` 但不误伤 `/bootleg`。
3. **符号链接零容忍**：路径本身或任何一段是符号链接即拒绝
   （`/var/log` 指向别处的情况真实存在）。
4. **策略分级沿用既有语义**：正在写入的日志必须 `truncate`
   （rsyslog 持句柄，unlink 后空间不释放且日志消失）；
   已轮转归档的文件才 `delete`。本表**不使用 recycle**——
   回收站在 `/volN`，跨分区搬运对解决系统分区空间问题没有帮助。

**刻意排除在自动化之外**（只提示不删，见 `systemBGradePaths` 与
`systemCommandRules`）：`/usr/share/{doc,man,locale}`、`/usr/local/go`、
`/usr/lib/firmware`、`/usr/share/AAVMF`、`/usr/lib/gcc`、`/usr/share/proj`、
`/usr/share/pocketsphinx`，以及由 apt 管理的 `/var/lib/apt/lists`。
理由：风险是"功能还在但依赖被删"，或绕过包管理器会让 dpkg 状态与磁盘不一致。

**内核与引导完全不在本表范围内**：`/boot`、`/usr/lib/modules`、
`/usr/lib/linux-image-*` 一律由 `routes/kernel.go` 的专用功能负责，
那里有 `uname -r` + `extlinux.conf` 的双重校验。`/boot/vmlinuz.old`
这个悬空链接虽是残留，也只作提示——引导分区上的任何删除都必须与内核
清理一并确认，不能走通用规则表。

**验证**：新增 `spaceclean_system_test.go`（9 个顶层测试），
路由层 `TestSpaceCleanSkipsPathsOutsideRules` 扩充了 13 条系统分区危险路径，
全部被正确拒绝。变异测试 `mutate_system_rules.py` **10/10 CAUGHT**：

| 变异 | 结果 |
| --- | --- |
| 拒绝名单失效（恒 false） | CAUGHT |
| 拒绝名单不再按路径段匹配（裸前缀误伤 `/bootleg`） | CAUGHT |
| 去掉连字符前缀处理（内核设备树变成可删） | CAUGHT |
| `matchSystemRule` 不再查拒绝名单 | CAUGHT |
| `matchSystemRule` 不再拒绝符号链接 | CAUGHT |
| 已轮转日志判定放宽（把正在写入的日志当可删） | CAUGHT |
| `DirectChildren` 不再排除目录 | CAUGHT |
| 日志策略 truncate → delete | CAUGHT |
| `anySymlinkComponentWith` 无法判定时放行 | CAUGHT |
| `anySymlinkComponentWith` 忽略组件链接 | CAUGHT |

> 又一条过程教训：`matchSystemRule` 里"查拒绝名单"最初**写了两遍**
> （循环前一次、循环内一次）。变异测试删掉其中一处时测试仍然全绿——
> 因为另一处还在。这说明重复的防御会**掩盖**单点失效。已删除冗余的那一处，
> 让这条规则只有一个必查点，变异随即被抓住。同理，
> "拒绝名单参与准入"这条属性一开始也没有测试覆盖（测试用的路径本来就
> 命中不了规则），补了"命中规则 + 在名单内须拒绝"与其对照用例才真正生效。


## 二、/vol1 顶层占用

| 目录 | 占用 | 文件数 | 性质 |
| --- | --- | --- | --- |
| `1000` | 21.411 GB | 26,212 | 用户主目录（含 pt 17.435 GB、Photos、media） |
| `@appshare` | 14.602 GB | 1,285 | 应用共享数据（qbittorrent/downloads 14.43 GB） |
| `@appdata` | 6.46 GB | 125,681 | 应用数据 |
| `@appcenter` | 5.252 GB | 55,973 | 应用安装目录（运行时） |
| `@sysappmeta` | 2.91 GB | 13,893 | 系统应用元数据 |
| `docker` | 1.866 GB | 87,733 | Docker 数据 |
| `@appmeta` | 1.187 GB | 5,659 | 应用元数据 |
| `appcenter-downloads` | 0.006 GB | 20 | 安装包残留 |
| `thumb` | 0.001 GB | 83 | 缩略图缓存 |
| `@appconf`/`@apptemp`/`@apphome`/`AppData`/`mediasrv.transcode`/`.cache.trim-sysrestore` | ≈ 0 | — | — |

## 三、可删除清单（按可回收大小排序）

> **重要更正**：本报告早期版本把 `home/.cache`、`config/.browser`、`pnpm-env`
> 列为"明确可删"，这是**错误的**。经逐目录核对（见第六节），这三者分别是
> Puppeteer/Chrome 真实浏览器二进制、内置 Chromium 安装目录、pnpm 工具本体，
> 删除会破坏对应应用。现已从清理规则表移除，只作为提示展示。

### A. 明确可删（可再生缓存，删除后自动重建）

| # | 路径 | 可回收 | 说明 |
| --- | --- | --- | --- |
| 1 | `/vol1/@appdata/deepseek.harness/pnpm-home/store` | 2150.4 MB | pnpm 内容寻址 store，已装依赖走硬链接引用 |
| 2 | `/vol1/@appdata/deepseek.harness/home/.cache/go-build` | 558.6 MB | Go 编译缓存，重新编译即重建 |
| 3 | `/vol1/1000/AppData/qinglong/dep_cache` | 305.8 MB | 容器依赖缓存，下次执行任务重装 |
| 4 | `/vol1/@appdata/deepseek.harness/home/.cache/pnpm` | 179.2 MB | pnpm 下载缓存 |
| 5 | `/vol1/@appdata/deepseek.harness/npm-cache/_cacache` | 138.4 MB | npm 下载缓存 |
| 6 | `/vol1/@appdata/deepseek.harness/home/.cache/pip` | 108.6 MB | pip 下载缓存 |
| 7 | `/vol1/@appdata/moviepilot/config/database_backup` | 67.2 MB | 数据库备份（移入回收站而非直接删） |
| 8 | `/vol1/@appdata/deepseek.harness/home/.cache/node-gyp` | 59.0 MB | node-gyp 编译缓存 |
| 9 | `/vol1/@appdata/deepseek.harness/npm-cache/_npx` | 37.0 MB | npx 临时安装缓存 |
| 10 | `/vol1/@appdata/moviepilot/config/logs/moviepilot.log` | 3.8 MB | 应用日志，由本应用**日志清理**功能处理（不在空间规则表内） |
| 11 | `/vol1/appcenter-downloads`（含 `logmanager-0.8.1.6-tpk`、`logmanager-0.8.3-tpk`） | 63.2 MB | 安装包解包残留，安装完成后无用 |
| 12 | `/vol1/@appdata/moviepilot/config/cache` | 5.4 MB | 应用缓存 |
| 13 | `/vol1/@appdata/deepseek.harness/npm-cache/_logs` | 2.4 MB | npm 调试日志 |
| 14 | `/vol1/thumb/*` | 0.6 MB | 缩略图缓存，浏览时自动重建 |
| 15 | `/vol1/@apptemp/*` | 7,817 B | 应用临时目录（运行中应用除外） |
| 16 | `/vol1/docker/containers/*/*-json.log` | 累计约 0.86 MB | 容器日志（**截断**而非删除） |

**小计：规则表约 3.6 GB；日志清理另有 3.8 MB。**

> **三处更正（第二次核对）**：
> 1. 早期版本把 `/vol1/1000/AppData/xiaoya/alist`（207.6 MB）列为可删，**这是错的**——
>    展开后是 `data/strm_internal.db`（206,716,928 B）与 `data/data.db`（10,936,320 B），
>    是小雅 Alist 的**索引数据库**，删掉要全量重建扫描；同目录另有 1,950 B 的
>    `quark_cookie.txt`、1,610 B 的 `uc_cookie.txt` 等登录凭证。它与
>    `qinglong/dep_cache` 同属容器数据，但 `dep_cache` 是纯下载依赖、`alist/` 不是。
>    已改列到「不可删除」表。
> 2. 早期版本把 `/vol1/@appdata/moviepilot/config/logs` 整体（11.1 MB）计入空间规则，
>    但**规则表里没有 `config/logs`**（只有 `config/cache` 与 `config/database_backup`）。
>    它实际由本应用的**日志清理**功能覆盖，且实测该目录下真正的日志只有
>    `moviepilot.log` 3,972,596 B，其余是 `plugins/` 子目录——所以可回收量从 11.1 MB
>    改为 3.8 MB。这类"日志算谁管"的混淆正是必须按机制分别统计的原因。
> 3. `qinglong/dep_cache` 实测 320,680,950 B（305.8 MB）、`appcenter-downloads`
>    实测 66,254,650 B（63.2 MB，因为上传的 0.8.3 包与其解包目录都还在），
>    与首次测量的数值不同。本表数值全部经 `file calc` 实测。


各 `home/.cache` 子项具体体积（合计 1559.5 MB，其中只有 905 MB 可删）：

| 子目录 | 体积 | 是否可删 |
| --- | --- | --- |
| `puppeteer` | 691,679,065 B | **否**，Puppeteer 下载的 Chrome 本体 |
| `go-build` | 558,601,112 B | 可删 |
| `pnpm` | 179,224,469 B | 可删 |
| `pip` | 108,598,016 B | 可删 |
| `node-gyp` | 58,952,429 B | 可删 |
| `billion-context` | 397,431 B | 未纳入规则 |
| `incus` | 0 B | — |
| `dconf` | 2 B | — |

### A2. 已卸载应用残留：本机实测为「零」

用户关心的另一类可清理项是**已卸载应用的残留**。本机 24 个已安装应用与磁盘上
全部 7 类应用目录、共 94 个子目录逐一比对（大小写不敏感），**孤儿数为 0**：

| 目录 | 子目录数 | 孤儿（不在已安装列表） |
| --- | --- | --- |
| `/vol1/@appdata` | 12 | **0** |
| `/vol1/@appcenter` | 19 | **0** |
| `/vol1/@appconf` | 11 | **0** |
| `/vol1/@apphome` | 11 | **0** |
| `/vol1/@apptemp` | 11 | **0** |
| `/vol1/@appmeta` | 22 | **0** |
| `/vol1/@appshare` | 8 | **0** |
| **合计** | **94** | **0** |

需要注意 `@appcenter` 里那 5 个 `fndesk_<时间戳>` 目录**不是残留**——它们都在
已安装列表里（各 0～0.2 MB，是同一应用的多份桌面快捷方式条目），真正的
`fndesk` 本体是 16.6 MB。若按"名字看着像重复"去删，会破坏桌面入口。

`/vol1/1000/AppData` 下 10 个容器数据目录（`qinglong`、`xiaoya`、`openlist`、
`transmission`、`moviepilot`、`decodev`、`Lucky`、`bash`、`icon`、
`docker-compose`）同样都不是应用残留：它们是 Docker 容器的挂载数据，
既不在应用中心列表里，也不受应用中心管理。其中只有 `qinglong/dep_cache`
（305.8 MB）属于可再生缓存并已纳入规则。

**但 `/vol1/appcenter-downloads` 有 63.2 MB 安装包残留**，这是本轮唯一实测到的
"用完即可删"类残留：

| 子项 | 体积 | 说明 |
| --- | --- | --- |
| `logmanager-0.8.3-tpk` | 22,128,784 B | 本次上传包的解包残留 |
| `logmanager-0.8.1.6-tpk` | 22,060,656 B | 上一次安装的解包残留 |
| `logmanager-0.8.3.fpk` | 22,065,210 B | 上传的安装包本体（已被规则 `appcenter-downloads/*` 覆盖） |
| `upload/` | 0 B | 空目录 |

结论：**本机"可清理"的大头不是已卸载应用残留（实为 0），而是可再生缓存与
安装包残留（约 3.6 GB）。** 已卸载残留扫描这条链路本身是完备的
（`ScanUninstalledLeftovers` 覆盖数据目录 + 失效符号链接 + `docker-*` 孤儿账号，
且已接入 UI 的「清理已卸载应用残留」），只是本机恰好没有可清理的对象。

### B. 不可删除（占用大，但属于系统或用户数据）

| 路径 | 占用 | 不可删原因 |
| --- | --- | --- |
| `/vol1/1000/pt` | 17.435 GB | 用户数据（PT 下载/做种） |
| `/vol1/@appshare/qbittorrent/downloads` | 14.43 GB | 用户下载内容（2 个种子） |
| `/vol1/@appcenter/trim.ai-runtime-rk3568` | 3.701 GB | 系统 AI 运行时（models 3038.8 MB、penv 605.8 MB、wheels 137.5 MB），删除会导致 AI 功能失效 |
| `/vol1/@sysappmeta/ai-manager` | 2.91 GB | 系统 AI 模型（txt2vec 1338.3 MB / img2vec 660.6 MB / face 367.7 MB），系统托管 |
| `/vol1/1000/AppData/moviepilot` | 1.828 GB | 应用持久化数据 |
| `/vol1/@appdata/deepseek.harness/home/.cache`（整体） | 1559.5 MB | 内含 691.7 MB 的 Puppeteer Chrome 本体，整体不可删 |
| `/vol1/@appdata/deepseek.harness/dsh-runtime/node_modules` | 849 MB | **运行期依赖，删除后应用无法启动**，需重装 |
| `/vol1/@appcenter/moviepilot/python` | 713.1 MB | 应用运行时 |
| `/vol1/@appdata/moviepilot/config/.browser` | 655.2 MB | **内置 Chromium 安装目录**（`cloakbrowser/chromium-146.0.7680.177.3`，672 个文件），删除后相关站点功能失效 |
| `/vol1/1000/Photos` | 0.727 GB | 用户照片 |
| `/vol1/1000/media` | 0.517 GB | 用户媒体 |
| `/vol1/@appdata/agent2api` | 432.0 MB | 应用数据 |
| `/vol1/@appdata/deepseek.harness/dsh-data/profiles` | 271.6 MB | 用户配置/会话数据 |
| `/vol1/1000/AppData/xiaoya/alist` | 207.6 MB | **小雅 Alist 索引数据库**（`data/strm_internal.db` 206,716,928 B + `data/data.db` 10,936,320 B），删后需全量重建；同目录另有 4 个 cookie/token 文本 |
| `/vol1/@appdata/deepseek.harness/pnpm-env` | 79.9 MB | **pnpm 工具本体**（`.bin/{pn,pnx,pnpm,pnpx}`），不是缓存 |
| `/vol1/@appdata/deepseek.harness/pnpm-env` | 79.9 MB | **pnpm 工具本体**（`.bin/{pn,pnx,pnpm,pnpx}`），不是缓存 |
| `/vol1/@appcenter/*`（其余已装运行时） | — | 已安装应用运行时 |

## 四、风险提示

1. **不要用「扫描全盘找大目录」的方式清理。** `/vol1/@appcenter/trim.ai-runtime-rk3568`
   和 `/vol1/@sysappmeta/ai-manager` 合计 6.6 GB，看起来最"肥"，但删掉会直接打断
   系统的 AI 能力。清理目标必须是**白名单规则表**。
2. **`dsh-runtime/node_modules`（849 MB）看似缓存，实为运行期依赖。** 删除后
   `deepseek.harness` 将无法启动，必须重装才能恢复。
3. **Docker 日志必须截断而不是删除。** 直接 unlink 后 dockerd 仍持有原 inode 继续写，
   空间不会立即释放，且日志从此丢失。截断可让容器无需重启即回收空间。
4. **回收站并不"立刻"释放空间**，要等 24 小时自动清空。因此可再生缓存直接删除，
   只有数据库备份这类需要保留可还原性的才移入回收站。
5. **`/vol1/thumb` 顶层看起来镜像了整个卷**（含 `@appdata`、`@appcenter` 等同名子目录），
   实际只是同名占位目录，真实数据是 `big/list/medium` 三层缩略图。实测体积 628,556 B（0.6 MB）。
6. **`@apptemp` 下含应用运行期临时区**，删除运行中应用的临时目录可能影响其工作，
   已通过 pid 存活探测排除（含本应用自己的 `@apptemp/logmanager`）。
7. **"名字像重复"不等于可删。** `@appcenter/fndesk_<时间戳>`（5 个）都在已安装列表里，
   是桌面快捷方式条目，不是旧版本残留；按名字相似去删会破坏桌面入口。
8. **"看着像应用残留"的目录可能其实是 Docker 容器数据。** `/vol1/1000/AppData/*`
   下 10 个目录都不在应用中心列表里，但它们是容器的挂载数据（`docker-compose` 目录里
   有 8 个 yml 可佐证）。对这类路径只清理明确可再生的子项（如 `dep_cache`），
   不整体删除。
9. **同为容器数据，性质可能完全相反。** `xiaoya/alist/data/strm_internal.db`（206.7 MB）
   是索引数据库，`qinglong/dep_cache`（305.8 MB）才是下载缓存——只有后者可删。
   判定必须落到具体子路径，不能停在应用级或容器级。

## 五、清理功能已集成的分类（15 个）

| 分类 ID | 名称 | 风险 | 策略 | 说明 |
| --- | --- | --- | --- | --- |
| `app-center-downloads` | 应用中心安装包 | 低 | 直接删除 | 安装完成后无用 |
| `pkg-cache` | 包管理器缓存 | 低 | 直接删除 | pnpm store / npm `_cacache` / `_npx` |
| `browser-cache` | 运行时下载缓存 | 低 | 直接删除 | home 下 pnpm / pip / node-gyp / go-build |
| `thumb-cache` | 缩略图缓存 | 低 | 直接删除 | 浏览时自动重建 |
| `app-temp` | 应用临时目录 | 低 | 直接删除 | `@apptemp`，运行中应用除外 |
| `docker-logs` | Docker 容器日志 | 低 | **截断** | 容器无需重启 |
| `app-cache` | 应用运行缓存 | 低 | 直接删除 | `config/cache` 等可重建缓存 |
| `container-cache` | 容器依赖缓存 | 中 | 直接删除 | 下次执行任务重装 |
| `db-backup` | 应用数据库备份 | 中 | **移入回收站** | 保留期内可还原 |
| `system-pkg-cache` | 系统包缓存（系统分区） | 低 | 直接删除 | apt `*.bin` 索引，`apt-get update` 重建 |
| `system-logs` | 系统日志（系统分区） | 低 | **截断 / 删除** | 写入中的截断；已轮转归档的删除 |
| `system-residue` | 系统零散残留 | 低 | 直接删除 | `*.dpkg-dist`、`drivedb.h.old` 等 |
| `system-pkg-mgr` | 需包管理器处理 | 中 | **仅提示** | `/var/lib/apt/lists`，给出建议命令 |
| `system-temp` | 系统临时目录 | 中 | 直接删除 | `/tmp`、`/var/tmp` 一级子项，双闸门（无进程占用 + 闲置 >24h） |
| `runtime-deps` | 应用运行期依赖 | 高 | **仅提示** | 删除会导致应用无法启动 |

四层安全设计：
1. **白名单规则表**（`spaceRules` + `systemPathRules`）：逐段精确匹配目录名，
   `*` 不跨路径分隔符；
2. **兜底拒绝名单**（`systemPathIsForbidden`）：系统分区上的 `/boot`、内核目录、
   `/etc/passwd`、`/var/lib/dpkg`、`/usr/trim` 等即使规则表被误改也永不接受；
3. **服务端二次校验**（`matchCleanablePath`）：客户端提交的每个路径都重新校验，
   不在规则内的一律返回 `status: "skipped"`——前端被绕过也删不掉规则外的文件；
4. **策略分级**：可再生缓存 `delete`、数据库备份 `recycle`、容器日志与系统日志 `truncate`。

`system-temp` 的准入不看文件名（`/tmp` 里任何进程都能创建任意名字），只看运行时事实：
**没有任何进程持有它**（`/proc/<pid>/{fd,cwd,exe}` 全量枚举，含 `(deleted)` 目标）
**且闲置超过 24 小时**。任一条件无法确认（例如读不到 `/proc`）一律拒绝删除。
这比 `systemd-tmpfiles` 只按 mtime 判断更保守——后者会删掉应用在两个步骤之间
短暂关闭的文件。

## 六、规则准确性核对（三次修正）

逐目录核对真实内容后，发现并从规则表中**移除**了三条会造成破坏的规则：

| # | 原规则 | 指向的真实内容 | 后果 | 处理 |
| --- | --- | --- | --- | --- |
| 1 | `@appdata/*/config/.browser` | `/vol1/@appdata/moviepilot/config/.browser` = `cloakbrowser/chromium-146.0.7680.177.3`，685,551,733 B / 672 个文件 | 删掉的是**真实 Chromium 安装目录**，不是缓存 | 移除规则，改为提示 |
| 2 | `@appdata/*/home/.cache` | `/vol1/@appdata/deepseek.harness/home/.cache` 1559.5 MB，其中 `puppeteer/` = 691,679,065 B 是 Puppeteer 下载的 **Chrome 二进制** | 同上 | 移除规则，拆成 4 条窄规则（`pnpm`/`pip`/`node-gyp`/`go-build`），其余改为提示 |
| 3 | `@appdata/*/pnpm-env` | `/vol1/@appdata/deepseek.harness/pnpm-env` = pnpm **工具本体**（`.bin/{pn,pnx,pnpm,pnpx}`） | 删掉后 pnpm 命令不可用 | 移除规则，改为提示 |

另外修正了 pid 定位：`moviepilot`、`qbittorrent`、`transmission` 三个应用在
`@appdata/<app>` 下**没有** pid 文件，pid 只存在于 `@apptemp/<app>`。
`appPidDirs` 现按三种真实布局查找：`@appdata/{app}`、`@apptemp/{app}`、
`var/apps/{app}`（旧版回退）。

以上三处均由变异测试锁定：把修正改回去会让测试失败。

## 七、验证方式

| 手段 | 覆盖内容 | 结果 |
| --- | --- | --- |
| 单元测试 | 规则匹配、路径校验、双闸门判定、`/proc` 解析 | 全绿 |
| **变异测试**（`.local-build/mutate_systemtmp.py`） | 对每道安全闸门注入一个"让它变得不安全"的改动，期望测试失败 | **16/16 全部被捕获** |
| 路由层测试 | 空列表/超限/非法 JSON 被拒；11 个危险路径逐条 `skipped` 且 `freed=0` | 全绿 |
| 交叉编译 | `GOOS=linux` amd64 + arm64 构建与 `go vet` | 均通过 |

变异测试的 16 项包括：把"读不到 `/proc`"当成"没人用"、只看条目自身不查内部文件、
去掉 24 小时闸门、接受任意深度子路径、`/tmpfoo` 被当成 `/tmp` 下条目、把 `(deleted)`
当成路径一部分、接受非绝对路径、pid 名用 `Atoi` 判断、没有 pid 时返回空集合、
某进程退出就整体放弃、规则展开跟随符号链接、接受不存在的中间段、把符号链接列为
清理目标，以及分区判定的三种失效方式（读不到设备号当成同分区、忽略根目录差异、
没有可比对象时也声称同分区）。

> 有一项变异最初**未被捕获**：`分区判定：没有可比对象时也声称同分区`。原因是该
> 断言所在测试在 Windows 上会因 `deviceOf("/")` 先返回错误而提前通过，`roots == 0`
> 这道闸门从未真正执行到。修正测试使其 stub 出可读的设备号后才真正覆盖。
> 这说明"测试通过"本身不等于"闸门被验证"。

交叉编译同样发现了 Windows 本地构建掩盖不了的问题：`deviceOf` 原实现用
`syscall.Statfs_t.Dev`，而 Linux 的 `Statfs_t` **没有** `Dev` 字段（只有 `Fsid`），
`GOOS=windows` 下完全看不出来。已改为 `stat(2)` 的 `st_dev`。

## 八、当前部署状态

- `.fpk` 已重新打包：`logmanager-0.8.3.fpk`，内部 UI 资源为
  `ui/assets/index-DQbNztaN.js`、`ui/assets/index-DjbGaRTy.css`，两个架构的
  server 二进制均为最新构建。
- **自动安装失败**：`app install-fpk` 返回 `{"code":10236}`。已用已知可用的
  `logmanager-0.8.1.6.fpk` 做过对照实验，返回**完全相同**的错误码，因此这是
  App Center 对"手动安装的第三方应用"（`manualInstall: true`、`sourceID: ""`）
  的限制，不是包本身的问题。试运行（`--dry-run`）对两者都能成功。
- 因此 0.8.3 **需要在飞牛 App Center 界面里手动安装**。当前设备上运行的是
  0.8.1.6（`status: running`），未被修改。
