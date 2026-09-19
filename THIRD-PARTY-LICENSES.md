# 第三方许可证与版权声明（Third-Party Licenses）

本项目（飞牛日志管理 / fnos-logmanager）自身以 **MIT License** 发布，见仓库根目录 `LICENSE`。

本文件归集本项目在构建与运行过程中使用的第三方依赖及其许可证信息，以满足再分发（如通过 fnpack 打包发布）时的许可证与版权声明义务。

> 说明：依赖版本取自 `app/server/go.mod` 与 `app/ui/package.json`。各依赖的许可证以其在对应版本中声明的 `LICENSE` 文件为准；本文件列出许可证类型并提供标准全文（见文末附录）。Apache-2.0 组件在再分发时须保留其 `LICENSE` 与 `NOTICE`（如适用）。

---

## 一、Go 依赖（app/server）

| 模块 | 版本 | 许可证 | 来源 |
|------|------|--------|------|
| github.com/gin-gonic/gin | v1.12.0 | MIT | https://github.com/gin-gonic/gin |
| github.com/gorilla/websocket | v1.5.3 | BSD-3-Clause | https://github.com/gorilla/websocket |
| github.com/robfig/cron/v3 | v3.0.1 | MIT | https://github.com/robfig/cron |
| github.com/skip2/go-qrcode | v0.0.0-20200617195104-da1b6568686e | MIT | https://github.com/skip2/go-qrcode |
| modernc.org/sqlite | v1.54.0 | BSD-3-Clause | https://modernc.org/sqlite |
| github.com/bytedance/gopkg | v0.1.4 | MIT | https://github.com/bytedance/gopkg |
| github.com/bytedance/sonic | v1.15.2 | Apache-2.0 | https://github.com/bytedance/sonic |
| github.com/bytedance/sonic/loader | v0.5.1 | Apache-2.0 | https://github.com/bytedance/sonic |
| github.com/cloudwego/base64x | v0.1.7 | Apache-2.0 | https://github.com/cloudwego/base64x |
| github.com/dustin/go-humanize | v1.0.1 | MIT | https://github.com/dustin/go-humanize |
| github.com/gabriel-vasile/mimetype | v1.4.15 | MIT | https://github.com/gabriel-vasile/mimetype |
| github.com/gin-contrib/sse | v1.1.1 | MIT | https://github.com/gin-contrib/sse |
| github.com/go-playground/locales | v0.14.1 | MIT | https://github.com/go-playground/locales |
| github.com/go-playground/universal-translator | v0.18.1 | MIT | https://github.com/go-playground/universal-translator |
| github.com/go-playground/validator/v10 | v10.30.3 | MIT | https://github.com/go-playground/validator |
| github.com/goccy/go-json | v0.10.6 | MIT | https://github.com/goccy/go-json |
| github.com/goccy/go-yaml | v1.19.2 | MIT | https://github.com/goccy/go-yaml |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause | https://github.com/google/uuid |
| github.com/json-iterator/go | v1.1.12 | MIT | https://github.com/json-iterator/go |
| github.com/klauspost/cpuid/v2 | v2.4.0 | MIT | https://github.com/klauspost/cpuid |
| github.com/leodido/go-urn | v1.5.0 | MIT | https://github.com/leodido/go-urn |
| github.com/mattn/go-isatty | v0.0.24 | MIT | https://github.com/mattn/go-isatty |
| github.com/modern-go/concurrent | (indirect) | Apache-2.0 | https://github.com/modern-go/concurrent |
| github.com/modern-go/reflect2 | v1.0.2 | Apache-2.0 | https://github.com/modern-go/reflect2 |
| github.com/ncruces/go-strftime | v1.0.0 | MIT | https://github.com/ncruces/go-strftime |
| github.com/pelletier/go-toml/v2 | v2.4.3 | MIT | https://github.com/pelletier/go-toml |
| github.com/quic-go/qpack | v0.6.0 | MIT | https://github.com/quic-go/qpack |
| github.com/quic-go/quic-go | v0.61.0 | MIT | https://github.com/quic-go/quic-go |
| github.com/remyoudompheng/bigfft | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause | https://github.com/remyoudompheng/bigfft |
| github.com/twitchyliquid64/golang-asm | v0.15.1 | MIT | https://github.com/twitchyliquid64/golang-asm |
| github.com/ugorji/go/codec | v1.3.1 | MIT | https://github.com/ugorji/go |
| go.mongodb.org/mongo-driver/v2 | v2.8.0 | Apache-2.0 | https://github.com/mongodb/mongo-go-driver |
| golang.org/x/arch | v0.29.0 | BSD-3-Clause | https://go.googlesource.com/arch |
| golang.org/x/crypto | v0.54.0 | BSD-3-Clause | https://go.googlesource.com/crypto |
| golang.org/x/net | v0.57.0 | BSD-3-Clause | https://go.googlesource.com/net |
| golang.org/x/sys | v0.47.0 | BSD-3-Clause | https://go.googlesource.com/sys |
| golang.org/x/text | v0.40.0 | BSD-3-Clause | https://go.googlesource.com/text |
| golang.org/x/tools | v0.48.0 | BSD-3-Clause | https://go.googlesource.com/tools |
| google.golang.org/protobuf | v1.36.11 | BSD-3-Clause（含 NOTICE） | https://github.com/protocolbuffers/protobuf |
| modernc.org/libc | v1.74.4 | BSD-3-Clause | https://modernc.org/libc |
| modernc.org/mathutil | v1.7.1 | BSD-3-Clause | https://modernc.org/mathutil |
| modernc.org/memory | v1.11.0 | BSD-3-Clause | https://modernc.org/memory |

---

## 二、前端依赖（app/ui）

| 包 | 版本 | 许可证 | 来源 |
|----|------|--------|------|
| @trimjs/web-app | ^0.4.2 | 许可证状态未明确（见下方说明） | https://www.npmjs.com/package/@trimjs/web-app |
| dompurify | ^3.3.3 | MPL-2.0 OR Apache-2.0（本项目择用 Apache-2.0） | https://github.com/cure53/DOMPurify |
| pinia | ^4.0.2 | MIT | https://github.com/vuejs/pinia |
| vue | ^3.5.31 | MIT | https://github.com/vuejs/core |

> 构建期依赖（vite、vitest、typescript、esbuild 等）不进入运行产物，未列入；如需完整清单请见 `app/ui/package.json`。

---

## 三、需特别说明的依赖

### 1. @trimjs/web-app（fnOS Web SDK）—— 许可证状态未明确

`@trimjs/web-app` 是飞牛 fnOS 提供的、用于在 fnOS 宿主环境（Web iframe / WebView）中构建三方应用的官方 SDK。经核查：

- 其 `package.json` **未声明 `license` 字段**；
- 发布包内**无 `LICENSE` 文件**（unpkg 返回 404）；
- npm 页面侧栏许可证显示为 `none`，但文档正文称其为 MIT。

即该依赖**当前未给出明确的许可证授权**。作为 fnOS 平台随官方开发工具/SDK 提供、用于构建 fnOS 三方应用的组件，实践中视为“随平台使用授权”，但严格从开源合规角度，其授权条款尚未书面确认。

**建议处理（请尽快落实其一）：**
- 向 fnOS 官方（飞牛开发者开放平台）确认 `@trimjs/web-app` 的许可证及再分发授权范围，并取得书面/仓库内 `LICENSE` 文件；或
- 在官方补齐 `LICENSE` 前，于本文件持续标注其“未明确授权”状态，并仅以“构建 fnOS 官方 SDK 应用”之目的使用、不另行再分发该 SDK 本身。

### 2. dompurify —— 双重许可择用

`dompurify` 以 `(MPL-2.0 OR Apache-2.0)` 双重许可发布。MPL-2.0 为文件级弱 copyleft，若仅作为依赖使用（未修改其源文件）亦可接受；为与本项目其他依赖保持一致并规避任何 copyleft 顾虑，**本项目明确择用 Apache-2.0**。

### 3. Apache-2.0 组件（再分发义务）

以下组件为 Apache-2.0：`bytedance/sonic`、`bytedance/sonic/loader`、`cloudwego/base64x`、`modern-go/concurrent`、`modern-go/reflect2`、`go.mongodb.org/mongo-driver/v2`，以及 `dompurify`（本项目择用 Apache-2.0）。再分发时须随产物附带 Apache-2.0 许可证全文（见附录）并保留其版权/属性声明；如组件含 `NOTICE` 文件亦须一并保留（`mongo-driver/v2` 在该版本无 `NOTICE`）。

---

## 附录：许可证标准全文

### MIT License

```
MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

### BSD 3-Clause License

```
Copyright (c) the respective copyright holders.
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software
   without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
```

### Apache License, Version 2.0

```
                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law or agreed to in writing, shall any
      Contributor be liable to You for damages, including any direct,
      indirect, special, incidental, or consequential damages of any
      character arising as a result of this License or out of the use or
      inability to use the Work.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor.

   END OF TERMS AND CONDITIONS

   Copyright [yyyy] [name of copyright owner]

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
```

---

*本文件由仓库维护者根据 `go.mod` / `package.json` 及上游仓库许可证信息整理，最后更新：2026-09-19。*
