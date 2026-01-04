# LLM Tool Usage Benchmark Report

## Metadata

- **Version**: kvit-coder baaf1fc (commit 20260103, built 2026-01-03)
- **Date**: 2026-01-03T22:08:04-06:00
- **Total Benchmarks**: 28
- **Total Runs**: 280

## Summary

| Class | Success Rate | Avg Time/Run |
|-------|--------------|--------------|
| C | 38% (15/40) | 14.5s |
| E | 50% (55/110) | 47.4s |
| R | 77% (23/30) | 4.9s |
| S | 30% (12/40) | 23.9s |
| W | 67% (40/60) | 19.4s |
| **Total** | **52% (145/280)** | **110.1s** |

## Detailed Statistics

### Per-Benchmark Summary

| Benchmark | Success | LLM Calls | Tokens | Generated | Context | Prompt Speed | Gen Speed | Cost | Duration |
|-----------|---------|-----------|--------|-----------|---------|--------------|-----------|------|----------|
| C1 | 50% | 2.5(±0.5) | 2928(±662) | 88(±34) | 1238 | 1102.8 t/s | 43.3 t/s | $0.0000 | 2.2s(±0.6) |
| C2 | 100% | 3.0(±0.4) | 3540(±581) | 89(±14) | 1251 | 1197.4 t/s | 37.7 t/s | $0.0000 | 2.6s(±0.5) |
| C3 | 0% | 2.6(±1.8) | 3357(±3250) | 118(±143) | 1272 | 973.4 t/s | 40.5 t/s | $0.0000 | 3.2s(±3.4) |
| C5 | 0% | 2.5(±1.5) | 3556(±3727) | 131(±130) | 1365 | 1274.2 t/s | 39.4 t/s | $0.0000 | 6.5s(±12.3) |
| E1 | 100% | 2.0 | 2314(±27) | 94(±28) | 1205 | 1012.2 t/s | 41.3 t/s | $0.0000 | 2.4s(±0.6) |
| E2 | 0% | 2.2(±0.4) | 2552(±492) | 91(±29) | 1204 | 997.4 t/s | 39.9 t/s | $0.0000 | 2.5s(±0.8) |
| E3 | 20% | 3.5(±1.6) | 4530(±2363) | 210(±125) | 1396 | 1250.0 t/s | 47.4 t/s | $0.0000 | 4.7s(±2.4) |
| E4 | 100% | 2.2(±0.6) | 2599(±911) | 105(±76) | 1230 | 1070.6 t/s | 44.5 t/s | $0.0000 | 2.6s(±1.5) |
| E5 | 100% | 2.0 | 2286(±4) | 71(±4) | 1180 | 971.4 t/s | 36.6 t/s | $0.0000 | 2.1s(±0.5) |
| E6 | 10% | 2.5(±1.2) | 3067(±1953) | 106(±86) | 1248 | 1122.5 t/s | 40.2 t/s | $0.0000 | 2.9s(±1.8) |
| E7 | 20% | 5.4(±4.1) | 8839(±8587) | 351(±359) | 1682 | 1177.6 t/s | 43.7 t/s | $0.0000 | 8.4s(±8.2) |
| E8 | 0% | 2.0 | 2280(±7) | 71(±7) | 1177 | 918.2 t/s | 38.3 t/s | $0.0000 | 2.1s(±0.5) |
| E9 | 50% | 2.1(±0.8) | 2412(±951) | 84(±40) | 1077 | 1072.9 t/s | 41.5 t/s | $0.0000 | 14.1s(±35.3) |
| E10 | 100% | 2.0 | 2307(±7) | 76(±7) | 1189 | 1076.3 t/s | 38.9 t/s | $0.0000 | 2.1s(±0.2) |
| E11 | 50% | 3.3(±1.0) | 4032(±1364) | 119(±36) | 1305 | 1266.6 t/s | 35.6 t/s | $0.0000 | 3.6s(±1.3) |
| R1 | 40% | 2.0 | 2262(±7) | 56(±7) | 1179 | 914.9 t/s | 45.0 t/s | $0.0000 | 1.4s(±0.1) |
| R2 | 90% | 2.1(±0.3) | 2512(±555) | 82(±6) | 1269 | 1457.4 t/s | 44.3 t/s | $0.0000 | 2.0s(±0.2) |
| R3 | 100% | 2.0 | 2244(±6) | 47(±6) | 1154 | 872.3 t/s | 38.6 t/s | $0.0000 | 1.5s(±0.4) |
| S1 | 10% | 2.6(±2.2) | 3401(±3647) | 134(±174) | 1199 | 1321.0 t/s | 42.7 t/s | $0.0000 | 15.3s(±35.0) |
| S2 | 0% | 2.0 | 2250(±2) | 47(±2) | 1152 | 945.2 t/s | 36.1 t/s | $0.0000 | 1.6s(±0.4) |
| S3 | 80% | 2.0 | 2345(±17) | 73(±8) | 1197 | 1169.0 t/s | 32.0 t/s | $0.0000 | 2.5s(±0.4) |
| S4 | 30% | 2.3(±0.6) | 2726(±749) | 142(±79) | 1231 | 1164.2 t/s | 33.4 t/s | $0.0000 | 4.5s(±2.4) |
| W1 | 100% | 2.0 | 2249(±2) | 49(±2) | 1154 | 869.1 t/s | 40.4 t/s | $0.0000 | 1.4s(±0.1) |
| W2 | 0% | 5.5(±0.8) | 6442(±1046) | 382(±100) | 1306 | 1076.2 t/s | 37.7 t/s | $0.0000 | 10.1s(±2.6) |
| W3 | 100% | 2.0 | 2245(±5) | 48(±5) | 1152 | 879.7 t/s | 40.9 t/s | $0.0000 | 1.3s(±0.1) |
| W4 | 0% | 3.1(±0.7) | 3787(±986) | 136(±55) | 1309 | 1125.6 t/s | 43.0 t/s | $0.0000 | 3.4s(±1.0) |
| W5 | 100% | 2.0 | 2234(±16) | 50(±16) | 1150 | 782.4 t/s | 43.6 t/s | $0.0000 | 1.3s(±0.3) |
| W6 | 100% | 2.0 | 2292(±15) | 72(±15) | 1182 | 1017.4 t/s | 39.1 t/s | $0.0000 | 1.9s(±0.2) |

### Per-Benchmark Details

#### C1

**✗ Search Then Read** | Find and read a file | 50% (5/10) | 2.5 calls | 2928 tokens | 2.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.5s | 3 | 3550 | 96 | 1291 | $0.0000 | Shell×2 |
| 2 | ✓ PASS | 2.5s | 3 | 3575 | 117 | 1291 | $0.0000 | Shell×2 |
| 3 | ✓ PASS | 2.4s | 3 | 3579 | 108 | 1303 | $0.0000 | Shell×2 |
| 4 | ✓ PASS | 2.9s | 3 | 3601 | 124 | 1317 | $0.0000 | Shell×2 |
| 5 | ✓ PASS | 3.1s | 3 | 3645 | 148 | 1355 | $0.0000 | Shell×2 |
| 6 | ✗ FAIL | 1.4s | 2 | 2250 | 47 | 1154 | $0.0000 | Shell |
| 7 | ✗ FAIL | 1.5s | 2 | 2253 | 50 | 1157 | $0.0000 | Shell |
| 8 | ✗ FAIL | 1.6s | 2 | 2257 | 54 | 1161 | $0.0000 | Shell |
| 9 | ✗ FAIL | 2.1s | 2 | 2289 | 68 | 1175 | $0.0000 | Shell |
| 10 | ✗ FAIL | 2.0s | 2 | 2284 | 65 | 1172 | $0.0000 | Shell |

**Failures:**

**Run 6**: output does not contain 'db.example.com'

**Run 7**: output does not contain 'db.example.com'

**Run 8**: output does not contain 'db.example.com'

**Run 9**: output does not contain 'db.example.com'

**Run 10**: output does not contain 'db.example.com'

---

#### C2

**✓ Read-Modify-Write** | Complete edit workflow | 100% (10/10) | 3.0 calls | 3540 tokens | 2.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.5s | 3 | 3532 | 90 | 1253 | $0.0000 | Shell×2 |
| 2 | ✓ PASS | 2.6s | 3 | 3537 | 97 | 1253 | $0.0000 | Shell×2 |
| 3 | ✓ PASS | 1.5s | 2 | 2267 | 56 | 1161 | $0.0000 | Shell |
| 4 | ✓ PASS | 2.5s | 3 | 3524 | 80 | 1243 | $0.0000 | Shell×2 |
| 5 | ✓ PASS | 2.6s | 3 | 3536 | 92 | 1255 | $0.0000 | Shell×2 |
| 6 | ✓ PASS | 2.6s | 3 | 3536 | 92 | 1255 | $0.0000 | Shell×2 |
| 7 | ✓ PASS | 3.7s | 3 | 3534 | 90 | 1253 | $0.0000 | Shell×2 |
| 8 | ✓ PASS | 2.6s | 3 | 3533 | 89 | 1252 | $0.0000 | Shell×2 |
| 9 | ✓ PASS | 2.7s | 3 | 3533 | 89 | 1252 | $0.0000 | Shell×2 |
| 10 | ✓ PASS | 3.1s | 4 | 4866 | 115 | 1334 | $0.0000 | Shell×3 |

---

#### C3

**✗ Search-Read-Edit** | Find, understand, and modify | 0% (0/10) | 2.6 calls | 3357 tokens | 3.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.8s | 2 | 2281 | 77 | 1182 | $0.0000 | Shell |
| 2 | ✗ FAIL | 1.7s | 2 | 2269 | 65 | 1170 | $0.0000 | Shell |
| 3 | ✗ FAIL | 1.6s | 2 | 2268 | 64 | 1169 | $0.0000 | Shell |
| 4 | ✗ FAIL | 13.1s | 8 | 13107 | 547 | 2142 | $0.0000 | Shell×7 |
| 5 | ✗ FAIL | 1.7s | 2 | 2267 | 63 | 1168 | $0.0000 | Shell |
| 6 | ✗ FAIL | 3.5s | 2 | 2296 | 92 | 1197 | $0.0000 | Shell |
| 7 | ✗ FAIL | 3.1s | 2 | 2275 | 71 | 1176 | $0.0000 | Shell |
| 8 | ✗ FAIL | 1.8s | 2 | 2271 | 67 | 1172 | $0.0000 | Shell |
| 9 | ✗ FAIL | 2.0s | 2 | 2278 | 74 | 1179 | $0.0000 | Shell |
| 10 | ✗ FAIL | 1.5s | 2 | 2262 | 58 | 1163 | $0.0000 | Shell |

**Failures:**

**Run 1**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


**Run 2**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


**Run 3**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


**Run 4**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -2,6 +2,6 @@
     newFunc();
 }
 
-function deprecatedFunc() {}
+function newFunc() {}
 function newFunc() {}
 

```


**Run 5**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


**Run 6**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


**Run 7**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


**Run 8**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


**Run 9**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


**Run 10**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function process() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 function handle() {
-    newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


---

#### C5

**✗ Needle in Haystack Search** | Find specific initialization among many usages | 0% (0/10) | 2.5 calls | 3556 tokens | 6.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.0s | 2 | 2292 | 73 | 1179 | $0.0000 | Shell |
| 2 | ✗ FAIL | 43.2s | 7 | 14736 | 505 | 2891 | $0.0000 | Shell×6 |
| 3 | ✗ FAIL | 2.0s | 2 | 2298 | 73 | 1179 | $0.0000 | Shell |
| 4 | ✗ FAIL | 1.9s | 2 | 2285 | 66 | 1172 | $0.0000 | Shell |
| 5 | ✗ FAIL | 2.6s | 2 | 2329 | 109 | 1215 | $0.0000 | Shell |
| 6 | ✗ FAIL | 2.0s | 2 | 2288 | 69 | 1175 | $0.0000 | Shell |
| 7 | ✗ FAIL | 2.3s | 2 | 2286 | 67 | 1173 | $0.0000 | Shell |
| 8 | ✗ FAIL | 2.0s | 2 | 2285 | 64 | 1170 | $0.0000 | Shell |
| 9 | ✗ FAIL | 4.7s | 2 | 2441 | 198 | 1304 | $0.0000 | Shell |
| 10 | ✗ FAIL | 2.5s | 2 | 2316 | 86 | 1192 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain 'wire.go'

**Run 2**: output does not contain 'wire.go'

**Run 3**: output does not contain 'wire.go'

**Run 4**: output does not contain 'wire.go'

**Run 5**: output does not contain 'wire.go'

**Run 6**: output does not contain 'wire.go'

**Run 7**: output does not contain 'wire.go'

**Run 8**: output does not contain 'wire.go'

**Run 9**: output does not contain 'wire.go'

**Run 10**: output does not contain 'wire.go'

---

#### E1

**✓ Single Line Replace** | Replace a single line | 100% (10/10) | 2.0 calls | 2314 tokens | 2.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.8s | 2 | 2291 | 69 | 1180 | $0.0000 | Shell |
| 2 | ✓ PASS | 2.7s | 2 | 2348 | 128 | 1239 | $0.0000 | Shell |
| 3 | ✓ PASS | 1.8s | 2 | 2301 | 79 | 1190 | $0.0000 | Shell |
| 4 | ✓ PASS | 3.6s | 2 | 2314 | 94 | 1205 | $0.0000 | Shell |
| 5 | ✓ PASS | 2.1s | 2 | 2307 | 87 | 1198 | $0.0000 | Shell |
| 6 | ✓ PASS | 3.1s | 2 | 2358 | 138 | 1249 | $0.0000 | Shell |
| 7 | ✓ PASS | 2.0s | 2 | 2296 | 76 | 1187 | $0.0000 | Shell |
| 8 | ✓ PASS | 3.1s | 2 | 2356 | 136 | 1247 | $0.0000 | Shell |
| 9 | ✓ PASS | 1.9s | 2 | 2289 | 69 | 1180 | $0.0000 | Shell |
| 10 | ✓ PASS | 1.8s | 2 | 2284 | 64 | 1175 | $0.0000 | Shell |

---

#### E2

**✗ Multi-line Insert** | Insert multiple lines at position | 0% (0/10) | 2.2 calls | 2552 tokens | 2.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.7s | 2 | 2288 | 63 | 1175 | $0.0000 | Shell |
| 2 | ✗ FAIL | 3.1s | 2 | 2310 | 85 | 1197 | $0.0000 | Shell |
| 3 | ✗ FAIL | 3.3s | 3 | 3423 | 120 | 1180 | $0.0000 | Shell |
| 4 | ✗ FAIL | 4.0s | 3 | 3637 | 138 | 1313 | $0.0000 | Shell×2 |
| 5 | ✗ FAIL | 1.9s | 2 | 2303 | 74 | 1186 | $0.0000 | Shell |
| 6 | ✗ FAIL | 1.8s | 2 | 2287 | 62 | 1174 | $0.0000 | Shell |
| 7 | ✗ FAIL | 2.0s | 2 | 2297 | 72 | 1184 | $0.0000 | Shell |
| 8 | ✗ FAIL | 3.1s | 2 | 2363 | 138 | 1250 | $0.0000 | Shell |
| 9 | ✗ FAIL | 2.5s | 2 | 2319 | 94 | 1206 | $0.0000 | Shell |
| 10 | ✗ FAIL | 1.9s | 2 | 2294 | 65 | 1177 | $0.0000 | Shell |

**Failures:**

**Run 1**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,7 +8,7 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
+nINSERTED
 INSERTED
 INSERTED
 FUNCTION K

```


**Run 2**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,7 +8,7 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
+nINSERTED
 INSERTED
 INSERTED
 FUNCTION K

```


**Run 3**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,11 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
+n
 INSERTED
 INSERTED
 INSERTED
+
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 4**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,9 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
+&
+&
+&
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 5**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,11 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
+n
 INSERTED
 INSERTED
 INSERTED
+
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 6**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,7 +8,7 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
+nINSERTED
 INSERTED
 INSERTED
 FUNCTION K

```


**Run 7**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,7 +8,7 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
+nINSERTED
 INSERTED
 INSERTED
 FUNCTION K

```


**Run 8**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,7 +8,7 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
+nINSERTED
 INSERTED
 INSERTED
 FUNCTION K

```


**Run 9**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,7 +8,7 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
+nINSERTED
 INSERTED
 INSERTED
 FUNCTION K

```


**Run 10**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,11 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
+n
 INSERTED
 INSERTED
 INSERTED
+
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


---

#### E3

**✗ Delete Lines** | Delete a range of lines | 20% (2/10) | 3.5 calls | 4530 tokens | 4.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 8.3s | 6 | 8344 | 357 | 1704 | $0.0000 | Shell×4 |
| 2 | ✗ FAIL | 1.9s | 2 | 2319 | 86 | 1216 | $0.0000 | Shell |
| 3 | ✓ PASS | 5.9s | 5 | 7177 | 306 | 1728 | $0.0000 | Shell×4 |
| 4 | ✗ FAIL | 4.3s | 4 | 4714 | 143 | 1203 | $0.0000 | Shell |
| 5 | ✗ FAIL | 3.1s | 2 | 2402 | 130 | 1280 | $0.0000 | Shell |
| 6 | ✗ FAIL | 8.9s | 6 | 8248 | 458 | 1657 | $0.0000 | Shell×4 |
| 7 | ✗ FAIL | 1.9s | 2 | 2310 | 70 | 1201 | $0.0000 | Shell |
| 8 | ✗ FAIL | 4.4s | 3 | 3598 | 205 | 1282 | $0.0000 | Shell |
| 9 | ✗ FAIL | 2.6s | 2 | 2358 | 87 | 1237 | $0.0000 | Shell |
| 10 | ✗ FAIL | 5.3s | 3 | 3832 | 260 | 1449 | $0.0000 | Shell×2 |

**Failures:**

**Run 2**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,10 @@
 function H
 function I
 function J
+function K
+function L
+function M
+function N
 function O
 function P
 function Q

```


**Run 4**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,10 @@
 function H
 function I
 function J
+function K
+function L
+function M
+function N
 function O
 function P
 function Q

```


**Run 5**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,10 @@
 function H
 function I
 function J
+function K
+function L
+function M
+function N
 function O
 function P
 function Q

```


**Run 6**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,10 @@
 function H
 function I
 function J
+function K
+function L
+function M
+function N
 function O
 function P
 function Q

```


**Run 7**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,10 @@
 function H
 function I
 function J
+function K
+function L
+function M
+function N
 function O
 function P
 function Q

```


**Run 8**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,10 @@
 function H
 function I
 function J
+function K
+function L
+function M
+function N
 function O
 function P
 function Q

```


**Run 9**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,10 @@
 function H
 function I
 function J
+function K
+function L
+function M
+function N
 function O
 function P
 function Q

```


**Run 10**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,10 @@
 function H
 function I
 function J
+function K
+function L
+function M
+function N
 function O
 function P
 function Q

```


---

#### E4

**✓ Boundary Edit** | Test boundary conditions | 100% (10/10) | 2.2 calls | 2599 tokens | 2.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.7s | 2 | 2282 | 67 | 1175 | $0.0000 | Shell |
| 2 | ✓ PASS | 3.1s | 2 | 2306 | 91 | 1199 | $0.0000 | Shell |
| 3 | ✓ PASS | 2.3s | 2 | 2321 | 106 | 1214 | $0.0000 | Shell |
| 4 | ✓ PASS | 1.8s | 2 | 2289 | 74 | 1182 | $0.0000 | Shell |
| 5 | ✓ PASS | 2.0s | 2 | 2289 | 74 | 1182 | $0.0000 | Shell |
| 6 | ✓ PASS | 1.8s | 2 | 2289 | 74 | 1182 | $0.0000 | Shell |
| 7 | ✓ PASS | 2.0s | 2 | 2288 | 73 | 1181 | $0.0000 | Shell |
| 8 | ✓ PASS | 6.9s | 4 | 5331 | 330 | 1607 | $0.0000 | Shell×3 |
| 9 | ✓ PASS | 2.1s | 2 | 2304 | 89 | 1197 | $0.0000 | Shell |
| 10 | ✓ PASS | 2.0s | 2 | 2289 | 74 | 1182 | $0.0000 | Shell |

---

#### E5

**✓ Replace All Occurrences** | Replace multiple occurrences | 100% (10/10) | 2.0 calls | 2286 tokens | 2.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.7s | 2 | 2283 | 68 | 1177 | $0.0000 | Shell |
| 2 | ✓ PASS | 1.8s | 2 | 2290 | 75 | 1184 | $0.0000 | Shell |
| 3 | ✓ PASS | 1.8s | 2 | 2290 | 75 | 1184 | $0.0000 | Shell |
| 4 | ✓ PASS | 2.8s | 2 | 2286 | 71 | 1180 | $0.0000 | Shell |
| 5 | ✓ PASS | 1.9s | 2 | 2290 | 75 | 1184 | $0.0000 | Shell |
| 6 | ✓ PASS | 1.6s | 2 | 2277 | 62 | 1171 | $0.0000 | Shell |
| 7 | ✓ PASS | 3.3s | 2 | 2286 | 71 | 1180 | $0.0000 | Shell |
| 8 | ✓ PASS | 1.9s | 2 | 2282 | 67 | 1176 | $0.0000 | Shell |
| 9 | ✓ PASS | 1.9s | 2 | 2288 | 73 | 1182 | $0.0000 | Shell |
| 10 | ✓ PASS | 1.8s | 2 | 2285 | 70 | 1179 | $0.0000 | Shell |

---

#### E6

**✗ Context-Specific Replace** | Replace with context for uniqueness | 10% (1/10) | 2.5 calls | 3067 tokens | 2.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.7s | 2 | 2279 | 70 | 1175 | $0.0000 | Shell |
| 2 | ✗ FAIL | 2.1s | 2 | 2298 | 89 | 1194 | $0.0000 | Shell |
| 3 | ✗ FAIL | 1.6s | 2 | 2274 | 68 | 1173 | $0.0000 | Shell |
| 4 | ✗ FAIL | 3.1s | 3 | 3515 | 89 | 1232 | $0.0000 | Shell×2 |
| 5 | ✓ PASS | 7.9s | 6 | 8822 | 359 | 1779 | $0.0000 | Shell×5 |
| 6 | ✗ FAIL | 2.2s | 2 | 2303 | 73 | 1196 | $0.0000 | Shell |
| 7 | ✗ FAIL | 2.6s | 2 | 2276 | 67 | 1172 | $0.0000 | Shell |
| 8 | ✗ FAIL | 2.1s | 2 | 2282 | 74 | 1179 | $0.0000 | Shell |
| 9 | ✗ FAIL | 4.2s | 2 | 2359 | 113 | 1224 | $0.0000 | Shell |
| 10 | ✗ FAIL | 1.5s | 2 | 2262 | 56 | 1161 | $0.0000 | Shell |

**Failures:**

**Run 1**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


**Run 2**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


**Run 3**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


**Run 4**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


**Run 6**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


**Run 7**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


**Run 8**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


**Run 9**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


**Run 10**: 
```diff
file values.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 const user = { value: 1 };
-const config = { value: 99 };
+const config = { value: 2 };
 const data = { value: 3 };
 

```


---

#### E7

**✗ Multi-line Block Replace** | Replace multi-line block | 20% (2/10) | 5.4 calls | 8839 tokens | 8.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.9s | 2 | 2291 | 70 | 1178 | $0.0000 | Shell |
| 2 | ✓ PASS | 11.3s | 10 | 16611 | 563 | 2180 | $0.0000 | Shell×9 |
| 3 | ✗ FAIL | 2.9s | 2 | 2295 | 74 | 1182 | $0.0000 | Shell |
| 4 | ✓ PASS | 25.4s | 13 | 26933 | 1070 | 2930 | $0.0000 | Shell×12 |
| 5 | ✗ FAIL | 4.7s | 4 | 4913 | 150 | 1334 | $0.0000 | Shell×3 |
| 6 | ✗ FAIL | 21.1s | 11 | 19802 | 899 | 2569 | $0.0000 | Shell×10 |
| 7 | ✗ FAIL | 10.6s | 6 | 8642 | 446 | 1884 | $0.0000 | Shell×5 |
| 8 | ✗ FAIL | 1.8s | 2 | 2279 | 58 | 1166 | $0.0000 | Shell |
| 9 | ✗ FAIL | 2.7s | 2 | 2333 | 112 | 1220 | $0.0000 | Shell |
| 10 | ✗ FAIL | 1.9s | 2 | 2292 | 71 | 1179 | $0.0000 | Shell |

**Failures:**

**Run 1**: 
```diff
file func.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,8 @@
 function calculate(): number {
-    return 42;
+    const a = 10;
+    const b = 20;
+    const c = a + b;
+    return c * 2;
 }
 
 function other(): number {

```


**Run 3**: 
```diff
file func.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,8 @@
 function calculate(): number {
-    return 42;
+    const a = 10;
+    const b = 20;
+    const c = a + b;
+    return c * 2;
 }
 
 function other(): number {

```


**Run 5**: 
```diff
file func.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,8 @@
 function calculate(): number {
-    return 42;
+    const a = 10;
+    const b = 20;
+    const c = a + b;
+    return c * 2;
 }
 
 function other(): number {

```


**Run 6**: 
```diff
file func.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,8 @@
 function calculate(): number {
-    return 42;
+    const a = 10;
+    const b = 20;
+    const c = a + b;
+    return c * 2;
 }
 
 function other(): number {

```


**Run 7**: 
```diff
file func.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,8 @@
 function calculate(): number {
-    return 42;
+    const a = 10;
+    const b = 20;
+    const c = a + b;
+    return c * 2;
 }
 
 function other(): number {

```


**Run 8**: 
```diff
file func.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,8 @@
 function calculate(): number {
-    return 42;
+    const a = 10;
+    const b = 20;
+    const c = a + b;
+    return c * 2;
 }
 
 function other(): number {

```


**Run 9**: 
```diff
file func.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,8 @@
 function calculate(): number {
-    return 42;
+    const a = 10;
+    const b = 20;
+    const c = a + b;
+    return c * 2;
 }
 
 function other(): number {

```


**Run 10**: 
```diff
file func.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,8 @@
 function calculate(): number {
-    return 42;
+    const a = 10;
+    const b = 20;
+    const c = a + b;
+    return c * 2;
 }
 
 function other(): number {

```


---

#### E8

**✗ No Match Handling** | Handle search text not found | 0% (0/10) | 2.0 calls | 2280 tokens | 2.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.8s | 2 | 2285 | 76 | 1182 | $0.0000 | Shell |
| 2 | ✗ FAIL | 1.9s | 2 | 2280 | 71 | 1177 | $0.0000 | Shell |
| 3 | ✗ FAIL | 1.7s | 2 | 2279 | 70 | 1176 | $0.0000 | Shell |
| 4 | ✗ FAIL | 2.0s | 2 | 2264 | 55 | 1161 | $0.0000 | Shell |
| 5 | ✗ FAIL | 1.8s | 2 | 2279 | 70 | 1176 | $0.0000 | Shell |
| 6 | ✗ FAIL | 3.5s | 2 | 2287 | 78 | 1184 | $0.0000 | Shell |
| 7 | ✗ FAIL | 2.1s | 2 | 2287 | 78 | 1184 | $0.0000 | Shell |
| 8 | ✗ FAIL | 1.9s | 2 | 2284 | 75 | 1181 | $0.0000 | Shell |
| 9 | ✗ FAIL | 1.8s | 2 | 2274 | 65 | 1171 | $0.0000 | Shell |
| 10 | ✗ FAIL | 2.0s | 2 | 2284 | 75 | 1181 | $0.0000 | Shell |

**Failures:**

**Run 1**: output contains 'successfully' but should not

**Run 2**: output contains 'successfully' but should not

**Run 3**: output contains 'successfully' but should not

**Run 4**: output contains 'successfully' but should not

**Run 5**: output contains 'successfully' but should not

**Run 6**: output contains 'successfully' but should not

**Run 7**: output contains 'successfully' but should not

**Run 8**: output contains 'successfully' but should not

**Run 9**: output contains 'successfully' but should not

**Run 10**: output contains 'successfully' but should not

---

#### E9

**✗ Empty Content** | Handle empty replacement | 50% (5/10) | 2.1 calls | 2412 tokens | 14.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.8s | 2 | 2300 | 72 | 1187 | $0.0000 | Shell |
| 2 | ✓ PASS | 1.9s | 2 | 2310 | 83 | 1198 | $0.0000 | Shell |
| 3 | ✓ PASS | 1.6s | 2 | 2296 | 69 | 1184 | $0.0000 | Shell |
| 4 | ✓ PASS | 3.0s | 3 | 3417 | 113 | 1193 | $0.0000 | Shell |
| 5 | ✗ FAIL | 1.5s | 2 | 2282 | 54 | 1169 | $0.0000 | Shell |
| 6 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 7 | ✓ PASS | 2.2s | 2 | 2311 | 84 | 1199 | $0.0000 | Shell |
| 8 | ✗ FAIL | 3.2s | 3 | 3422 | 117 | 1197 | $0.0000 | Shell |
| 9 | ✗ FAIL | 4.0s | 3 | 3469 | 162 | 1242 | $0.0000 | Shell |
| 10 | ✓ PASS | 2.1s | 2 | 2310 | 83 | 1198 | $0.0000 | Shell |

**Failures:**

**Run 1**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 LINE B
 LINE C
 LINE D
-
+ 
 LINE F
 LINE G
 LINE H

```


**Run 5**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 LINE B
 LINE C
 LINE D
-
+ 
 LINE F
 LINE G
 LINE H

```


**Run 6**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 LINE B
 LINE C
 LINE D
-
+LINE E
 LINE F
 LINE G
 LINE H

```


**Run 8**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 LINE B
 LINE C
 LINE D
-
+ 
 LINE F
 LINE G
 LINE H

```


**Run 9**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 LINE B
 LINE C
 LINE D
-
+LINE E
 LINE F
 LINE G
 LINE H

```


---

#### E10

**✓ Special Characters** | Handle special characters in replacement | 100% (10/10) | 2.0 calls | 2307 tokens | 2.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.1s | 2 | 2311 | 80 | 1193 | $0.0000 | Shell |
| 2 | ✓ PASS | 1.7s | 2 | 2298 | 67 | 1180 | $0.0000 | Shell |
| 3 | ✓ PASS | 2.0s | 2 | 2316 | 85 | 1198 | $0.0000 | Shell |
| 4 | ✓ PASS | 2.2s | 2 | 2321 | 90 | 1203 | $0.0000 | Shell |
| 5 | ✓ PASS | 2.0s | 2 | 2308 | 77 | 1190 | $0.0000 | Shell |
| 6 | ✓ PASS | 2.4s | 2 | 2299 | 68 | 1181 | $0.0000 | Shell |
| 7 | ✓ PASS | 2.1s | 2 | 2302 | 71 | 1184 | $0.0000 | Shell |
| 8 | ✓ PASS | 1.9s | 2 | 2298 | 67 | 1180 | $0.0000 | Shell |
| 9 | ✓ PASS | 2.1s | 2 | 2308 | 77 | 1190 | $0.0000 | Shell |
| 10 | ✓ PASS | 2.1s | 2 | 2309 | 78 | 1191 | $0.0000 | Shell |

---

#### E11

**✗ Indentation Preservation** | Maintain correct indentation (critical for Python) | 50% (5/10) | 3.3 calls | 4032 tokens | 3.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 3.0s | 3 | 3614 | 105 | 1280 | $0.0000 | Shell×2 |
| 2 | ✓ PASS | 2.9s | 3 | 3610 | 100 | 1275 | $0.0000 | Shell×2 |
| 3 | ✗ FAIL | 4.5s | 5 | 6410 | 177 | 1478 | $0.0000 | Shell×4 |
| 4 | ✓ PASS | 6.3s | 5 | 6429 | 174 | 1484 | $0.0000 | Shell×4 |
| 5 | ✗ FAIL | 1.9s | 2 | 2320 | 69 | 1192 | $0.0000 | Shell |
| 6 | ✓ PASS | 5.3s | 4 | 4761 | 166 | 1292 | $0.0000 | Shell×2 |
| 7 | ✗ FAIL | 2.4s | 2 | 2343 | 92 | 1215 | $0.0000 | Shell |
| 8 | ✓ PASS | 3.2s | 3 | 3610 | 100 | 1275 | $0.0000 | Shell×2 |
| 9 | ✓ PASS | 3.3s | 3 | 3610 | 100 | 1275 | $0.0000 | Shell×2 |
| 10 | ✗ FAIL | 3.2s | 3 | 3618 | 107 | 1282 | $0.0000 | Shell×2 |

**Failures:**

**Run 1**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -2,6 +2,6 @@
     if True:
         process_data()
         validate_input()
-        transform_data()
+    transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

**Run 3**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -2,6 +2,6 @@
     if True:
         process_data()
         validate_input()
-        transform_data()
+transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

**Run 5**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -2,6 +2,6 @@
     if True:
         process_data()
         validate_input()
-        transform_data()
+transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

**Run 7**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -2,6 +2,6 @@
     if True:
         process_data()
         validate_input()
-        transform_data()
+transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

**Run 10**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -2,6 +2,6 @@
     if True:
         process_data()
         validate_input()
-        transform_data()
+    transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

---

#### R1

**✗ Simple File Read** | Read an entire small file | 40% (4/10) | 2.0 calls | 2262 tokens | 1.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.5s | 2 | 2272 | 66 | 1189 | $0.0000 | Shell |
| 2 | ✗ FAIL | 1.4s | 2 | 2265 | 59 | 1182 | $0.0000 | Shell |
| 3 | ✗ FAIL | 1.3s | 2 | 2253 | 47 | 1170 | $0.0000 | Shell |
| 4 | ✓ PASS | 1.6s | 2 | 2267 | 61 | 1184 | $0.0000 | Shell |
| 5 | ✓ PASS | 1.5s | 2 | 2268 | 62 | 1185 | $0.0000 | Shell |
| 6 | ✓ PASS | 1.5s | 2 | 2267 | 61 | 1184 | $0.0000 | Shell |
| 7 | ✗ FAIL | 1.2s | 2 | 2249 | 43 | 1166 | $0.0000 | Shell |
| 8 | ✗ FAIL | 1.4s | 2 | 2257 | 51 | 1174 | $0.0000 | Shell |
| 9 | ✗ FAIL | 1.3s | 2 | 2258 | 52 | 1175 | $0.0000 | Shell |
| 10 | ✓ PASS | 1.5s | 2 | 2265 | 59 | 1182 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain 'db_host=localhost'

**Run 2**: output does not contain 'db_host=localhost'

**Run 3**: output does not contain 'db_host=localhost'

**Run 7**: output does not contain 'db_host=localhost'

**Run 8**: output does not contain 'db_host=localhost'

**Run 9**: output does not contain 'db_host=localhost'

---

#### R2

**✗ Truncation Recovery** | Handle truncated output by chunked reading | 90% (9/10) | 2.1 calls | 2512 tokens | 2.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.9s | 3 | 4176 | 69 | 1597 | $0.0000 | Shell×2 |
| 2 | ✓ PASS | 1.9s | 2 | 2331 | 88 | 1237 | $0.0000 | Shell |
| 3 | ✓ PASS | 1.9s | 2 | 2326 | 83 | 1232 | $0.0000 | Shell |
| 4 | ✓ PASS | 1.7s | 2 | 2323 | 80 | 1229 | $0.0000 | Shell |
| 5 | ✓ PASS | 2.6s | 2 | 2331 | 88 | 1237 | $0.0000 | Shell |
| 6 | ✓ PASS | 1.9s | 2 | 2323 | 78 | 1227 | $0.0000 | Shell |
| 7 | ✓ PASS | 2.0s | 2 | 2329 | 86 | 1235 | $0.0000 | Shell |
| 8 | ✓ PASS | 2.0s | 2 | 2329 | 86 | 1235 | $0.0000 | Shell |
| 9 | ✓ PASS | 1.8s | 2 | 2317 | 74 | 1223 | $0.0000 | Shell |
| 10 | ✓ PASS | 2.2s | 2 | 2332 | 89 | 1238 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

---

#### R3

**✓ Relative vs Absolute Path** | Correct path resolution | 100% (10/10) | 2.0 calls | 2244 tokens | 1.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.2s | 2 | 2248 | 51 | 1158 | $0.0000 | Shell |
| 2 | ✓ PASS | 1.2s | 2 | 2245 | 48 | 1155 | $0.0000 | Shell |
| 3 | ✓ PASS | 1.3s | 2 | 2247 | 50 | 1157 | $0.0000 | Shell |
| 4 | ✓ PASS | 1.2s | 2 | 2245 | 48 | 1155 | $0.0000 | Shell |
| 5 | ✓ PASS | 1.3s | 2 | 2236 | 39 | 1146 | $0.0000 | Shell |
| 6 | ✓ PASS | 1.4s | 2 | 2250 | 53 | 1160 | $0.0000 | Shell |
| 7 | ✓ PASS | 2.0s | 2 | 2236 | 39 | 1146 | $0.0000 | Shell |
| 8 | ✓ PASS | 2.4s | 2 | 2247 | 50 | 1157 | $0.0000 | Shell |
| 9 | ✓ PASS | 1.2s | 2 | 2236 | 39 | 1146 | $0.0000 | Shell |
| 10 | ✓ PASS | 1.5s | 2 | 2252 | 55 | 1162 | $0.0000 | Shell |

---

#### S1

**✗ Simple Pattern Search** | Find a specific function definition | 10% (1/10) | 2.6 calls | 3401 tokens | 15.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 11.9s | 9 | 13997 | 632 | 2168 | $0.0000 | Shell×8 |
| 2 | ✗ FAIL | 2.1s | 2 | 2300 | 91 | 1198 | $0.0000 | Shell |
| 3 | ✗ FAIL | 1.8s | 2 | 2269 | 60 | 1167 | $0.0000 | Shell |
| 4 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 5 | ✗ FAIL | 1.5s | 2 | 2264 | 55 | 1162 | $0.0000 | Shell |
| 6 | ✗ FAIL | 2.5s | 2 | 2327 | 98 | 1205 | $0.0000 | Shell |
| 7 | ✗ FAIL | 1.8s | 2 | 2274 | 65 | 1172 | $0.0000 | Shell |
| 8 | ✗ FAIL | 3.4s | 2 | 2278 | 69 | 1176 | $0.0000 | Shell |
| 9 | ✗ FAIL | 6.2s | 3 | 4038 | 210 | 1580 | $0.0000 | Shell×2 |
| 10 | ✗ FAIL | 1.7s | 2 | 2266 | 57 | 1164 | $0.0000 | Shell |

**Failures:**

**Run 2**: output does not contain 'func calculateTotal(items []int)'

**Run 3**: output does not contain 'func calculateTotal(items []int)'

**Run 4**: output does not contain 'func calculateTotal(items []int)'

**Run 5**: output does not contain 'func calculateTotal(items []int)'

**Run 6**: output does not contain 'func calculateTotal(items []int)'

**Run 7**: output does not contain 'func calculateTotal(items []int)'

**Run 8**: output does not contain 'func calculateTotal(items []int)'

**Run 9**: output does not contain 'func calculateTotal(items []int)'

**Run 10**: output does not contain 'func calculateTotal(items []int)'

---

#### S2

**✗ Multi-Pattern Search** | Find multiple related items | 0% (0/10) | 2.0 calls | 2250 tokens | 1.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.5s | 2 | 2251 | 48 | 1153 | $0.0000 | Shell |
| 2 | ✗ FAIL | 1.4s | 2 | 2254 | 51 | 1156 | $0.0000 | Shell |
| 3 | ✗ FAIL | 1.4s | 2 | 2251 | 48 | 1153 | $0.0000 | Shell |
| 4 | ✗ FAIL | 2.9s | 2 | 2248 | 45 | 1150 | $0.0000 | Shell |
| 5 | ✗ FAIL | 1.4s | 2 | 2251 | 48 | 1153 | $0.0000 | Shell |
| 6 | ✗ FAIL | 1.4s | 2 | 2251 | 48 | 1153 | $0.0000 | Shell |
| 7 | ✗ FAIL | 1.4s | 2 | 2248 | 45 | 1150 | $0.0000 | Shell |
| 8 | ✗ FAIL | 1.5s | 2 | 2248 | 45 | 1150 | $0.0000 | Shell |
| 9 | ✗ FAIL | 1.4s | 2 | 2248 | 45 | 1150 | $0.0000 | Shell |
| 10 | ✗ FAIL | 1.5s | 2 | 2251 | 48 | 1153 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain '7'

**Run 2**: output does not contain '7'

**Run 3**: output does not contain '7'

**Run 4**: output does not contain '7'

**Run 5**: output does not contain '7'

**Run 6**: output does not contain '7'

**Run 7**: output does not contain '7'

**Run 8**: output does not contain '7'

**Run 9**: output does not contain '7'

**Run 10**: output does not contain '7'

---

#### S3

**✗ Search with File Filtering** | Search in specific file types | 80% (8/10) | 2.0 calls | 2345 tokens | 2.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.7s | 2 | 2366 | 84 | 1206 | $0.0000 | Shell |
| 2 | ✓ PASS | 2.7s | 2 | 2366 | 84 | 1206 | $0.0000 | Shell |
| 3 | ✓ PASS | 2.6s | 2 | 2366 | 84 | 1206 | $0.0000 | Shell |
| 4 | ✓ PASS | 2.2s | 2 | 2330 | 66 | 1188 | $0.0000 | Shell |
| 5 | ✓ PASS | 3.3s | 2 | 2342 | 71 | 1195 | $0.0000 | Shell |
| 6 | ✗ FAIL | 2.2s | 2 | 2334 | 68 | 1194 | $0.0000 | Shell |
| 7 | ✓ PASS | 2.4s | 2 | 2342 | 72 | 1194 | $0.0000 | Shell |
| 8 | ✓ PASS | 2.7s | 2 | 2358 | 79 | 1203 | $0.0000 | Shell |
| 9 | ✓ PASS | 2.3s | 2 | 2330 | 65 | 1189 | $0.0000 | Shell |
| 10 | ✗ FAIL | 2.0s | 2 | 2319 | 61 | 1187 | $0.0000 | Shell |

**Failures:**

**Run 6**: output contains 'helpers.go' but should not

**Run 10**: output contains 'helpers.go' but should not

---

#### S4

**✗ Search Truncation Recovery** | Handle large search results requiring refinement | 30% (3/10) | 2.3 calls | 2726 tokens | 4.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.6s | 2 | 2266 | 61 | 1168 | $0.0000 | Shell |
| 2 | ✗ FAIL | 4.4s | 2 | 2366 | 116 | 1219 | $0.0000 | Shell |
| 3 | ✗ FAIL | 8.4s | 4 | 4681 | 295 | 1237 | $0.0000 | Shell |
| 4 | ✗ FAIL | 1.9s | 2 | 2257 | 52 | 1159 | $0.0000 | Shell |
| 5 | ✗ FAIL | 3.5s | 2 | 2396 | 140 | 1265 | $0.0000 | Shell |
| 6 | ✗ FAIL | 6.1s | 3 | 3552 | 196 | 1268 | $0.0000 | Shell |
| 7 | ✓ PASS | 9.1s | 2 | 2698 | 265 | 1376 | $0.0000 | Shell |
| 8 | ✗ FAIL | 2.7s | 2 | 2321 | 89 | 1191 | $0.0000 | Shell |
| 9 | ✓ PASS | 3.7s | 2 | 2391 | 119 | 1227 | $0.0000 | Shell |
| 10 | ✓ PASS | 2.7s | 2 | 2332 | 88 | 1196 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain 'folder3'

**Run 2**: output does not contain 'folder3'

**Run 3**: output does not contain 'folder3'

**Run 4**: output does not contain 'folder3'

**Run 5**: output does not contain 'folder3'

**Run 6**: output does not contain 'folder3'

**Run 8**: output does not contain 'folder3'

---

#### W1

**✓ Simple File Creation** | Create a new file with content | 100% (10/10) | 2.0 calls | 2249 tokens | 1.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.3s | 2 | 2248 | 48 | 1153 | $0.0000 | Shell |
| 2 | ✓ PASS | 1.3s | 2 | 2251 | 51 | 1156 | $0.0000 | Shell |
| 3 | ✓ PASS | 1.3s | 2 | 2248 | 48 | 1153 | $0.0000 | Shell |
| 4 | ✓ PASS | 1.3s | 2 | 2248 | 48 | 1153 | $0.0000 | Shell |
| 5 | ✓ PASS | 1.5s | 2 | 2248 | 48 | 1153 | $0.0000 | Shell |
| 6 | ✓ PASS | 1.4s | 2 | 2252 | 52 | 1157 | $0.0000 | Shell |
| 7 | ✓ PASS | 1.3s | 2 | 2248 | 48 | 1153 | $0.0000 | Shell |
| 8 | ✓ PASS | 1.4s | 2 | 2248 | 48 | 1153 | $0.0000 | Shell |
| 9 | ✓ PASS | 1.4s | 2 | 2251 | 51 | 1156 | $0.0000 | Shell |
| 10 | ✓ PASS | 1.3s | 2 | 2248 | 48 | 1153 | $0.0000 | Shell |

---

#### W2

**✗ Multi-line Content** | Write file with multiple lines and formatting | 0% (0/10) | 5.5 calls | 6442 tokens | 10.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 7.7s | 5 | 5741 | 268 | 1236 | $0.0000 | Shell |
| 2 | ✗ FAIL | 12.2s | 7 | 8338 | 508 | 1314 | $0.0000 | Shell |
| 3 | ✗ FAIL | 8.5s | 5 | 5795 | 324 | 1295 | $0.0000 | Shell |
| 4 | ✗ FAIL | 9.0s | 5 | 5806 | 333 | 1312 | $0.0000 | Shell |
| 5 | ✗ FAIL | 9.4s | 5 | 5817 | 346 | 1319 | $0.0000 | Shell |
| 6 | ✗ FAIL | 8.4s | 5 | 5786 | 320 | 1293 | $0.0000 | Shell |
| 7 | ✗ FAIL | 17.1s | 7 | 8445 | 615 | 1372 | $0.0000 | Shell |
| 8 | ✗ FAIL | 9.4s | 5 | 5816 | 345 | 1311 | $0.0000 | Shell |
| 9 | ✗ FAIL | 10.3s | 6 | 7073 | 425 | 1299 | $0.0000 | Shell |
| 10 | ✗ FAIL | 9.2s | 5 | 5806 | 340 | 1306 | $0.0000 | Shell |

**Failures:**

**Run 1**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 2**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 3**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 4**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 5**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 6**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 7**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 8**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 9**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 10**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-iquest/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


---

#### W3

**✓ Overwrite Existing** | Handle overwrite of existing file | 100% (10/10) | 2.0 calls | 2245 tokens | 1.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.4s | 2 | 2255 | 58 | 1162 | $0.0000 | Shell |
| 2 | ✓ PASS | 1.2s | 2 | 2240 | 43 | 1147 | $0.0000 | Shell |
| 3 | ✓ PASS | 1.1s | 2 | 2240 | 43 | 1147 | $0.0000 | Shell |
| 4 | ✓ PASS | 1.5s | 2 | 2246 | 49 | 1153 | $0.0000 | Shell |
| 5 | ✓ PASS | 1.2s | 2 | 2242 | 45 | 1149 | $0.0000 | Shell |
| 6 | ✓ PASS | 1.2s | 2 | 2240 | 43 | 1147 | $0.0000 | Shell |
| 7 | ✓ PASS | 1.5s | 2 | 2251 | 54 | 1158 | $0.0000 | Shell |
| 8 | ✓ PASS | 1.4s | 2 | 2245 | 48 | 1152 | $0.0000 | Shell |
| 9 | ✓ PASS | 1.3s | 2 | 2246 | 49 | 1153 | $0.0000 | Shell |
| 10 | ✓ PASS | 1.2s | 2 | 2243 | 46 | 1150 | $0.0000 | Shell |

---

#### W4

**✗ Special Characters** | Handle special characters in content | 0% (0/10) | 3.1 calls | 3787 tokens | 3.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.7s | 2 | 2295 | 65 | 1180 | $0.0000 | Shell |
| 2 | ✗ FAIL | 4.9s | 4 | 5126 | 238 | 1449 | $0.0000 | Shell×3 |
| 3 | ✗ FAIL | 3.4s | 3 | 3651 | 137 | 1302 | $0.0000 | Shell×2 |
| 4 | ✗ FAIL | 3.3s | 3 | 3575 | 105 | 1278 | $0.0000 | Shell×2 |
| 5 | ✗ FAIL | 3.5s | 3 | 3619 | 141 | 1307 | $0.0000 | Shell×2 |
| 6 | ✗ FAIL | 4.0s | 3 | 3596 | 126 | 1299 | $0.0000 | Shell×2 |
| 7 | ✗ FAIL | 4.2s | 4 | 5061 | 176 | 1392 | $0.0000 | Shell×3 |
| 8 | ✗ FAIL | 1.8s | 2 | 2304 | 64 | 1192 | $0.0000 | Shell |
| 9 | ✗ FAIL | 2.8s | 3 | 3553 | 96 | 1263 | $0.0000 | Shell×2 |
| 10 | ✗ FAIL | 4.8s | 4 | 5092 | 210 | 1427 | $0.0000 | Shell×3 |

**Failures:**

**Run 1**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, "double", `backticks`, $var
 

```


**Run 2**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, double, \`backticks\`,
 

```


**Run 3**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, double, `backticks`,
 

```


**Run 4**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, double, ,
 

```


**Run 5**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, double, `backticks`,
 

```


**Run 6**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, double, ,
 

```


**Run 7**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, double, \`backticks\`, \$var
 

```


**Run 8**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, double, ,
 

```


**Run 9**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, "double", `backticks`, $var
 

```


**Run 10**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,2 @@
-'quotes', "double", `backticks`, $var
+quotes, double, \`backticks\`, \$var
 

```


---

#### W5

**✓ Empty File Creation** | Create an empty file | 100% (10/10) | 2.0 calls | 2234 tokens | 1.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.1s | 2 | 2222 | 38 | 1138 | $0.0000 | Shell |
| 2 | ✓ PASS | 1.5s | 2 | 2253 | 69 | 1169 | $0.0000 | Shell |
| 3 | ✓ PASS | 1.0s | 2 | 2219 | 35 | 1135 | $0.0000 | Shell |
| 4 | ✓ PASS | 1.4s | 2 | 2245 | 61 | 1161 | $0.0000 | Shell |
| 5 | ✓ PASS | 1.2s | 2 | 2230 | 46 | 1146 | $0.0000 | Shell |
| 6 | ✓ PASS | 1.3s | 2 | 2222 | 38 | 1138 | $0.0000 | Shell |
| 7 | ✓ PASS | 1.9s | 2 | 2266 | 82 | 1182 | $0.0000 | Shell |
| 8 | ✓ PASS | 1.0s | 2 | 2217 | 33 | 1133 | $0.0000 | Shell |
| 9 | ✓ PASS | 1.0s | 2 | 2218 | 34 | 1134 | $0.0000 | Shell |
| 10 | ✓ PASS | 1.4s | 2 | 2245 | 61 | 1161 | $0.0000 | Shell |

---

#### W6

**✓ Path with Spaces** | Handle paths with spaces | 100% (10/10) | 2.0 calls | 2292 tokens | 1.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.7s | 2 | 2289 | 70 | 1180 | $0.0000 | Shell |
| 2 | ✓ PASS | 1.8s | 2 | 2289 | 70 | 1180 | $0.0000 | Shell |
| 3 | ✓ PASS | 2.4s | 2 | 2316 | 94 | 1204 | $0.0000 | Shell |
| 4 | ✓ PASS | 1.6s | 2 | 2261 | 42 | 1152 | $0.0000 | Shell |
| 5 | ✓ PASS | 1.8s | 2 | 2288 | 66 | 1176 | $0.0000 | Shell |
| 6 | ✓ PASS | 2.0s | 2 | 2289 | 70 | 1180 | $0.0000 | Shell |
| 7 | ✓ PASS | 1.9s | 2 | 2289 | 66 | 1176 | $0.0000 | Shell |
| 8 | ✓ PASS | 2.3s | 2 | 2316 | 97 | 1207 | $0.0000 | Shell |
| 9 | ✓ PASS | 1.9s | 2 | 2290 | 68 | 1178 | $0.0000 | Shell |
| 10 | ✓ PASS | 1.8s | 2 | 2293 | 72 | 1182 | $0.0000 | Shell |

---

## Failure Analysis

| Benchmark | Run | Errors | Last Tool Call |
|-----------|-----|--------|----------------|
| S2 | 1 | output does not contain '7' | Shell |
| S4 | 1 | output does not contain 'folder3' | Shell |
| R1 | 1 | output does not contain 'db_host=localhost' | Shell |
| R2 | 1 | output does not contain 'SECTION_ALPHA'; output... | Shell |
| W2 | 1 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 1 | file special.txt content does not match expecte... | Shell |
| E2 | 1 | file file.txt content does not match expected:
... | Shell |
| E6 | 1 | file values.ts content does not match expected:... | Shell |
| E7 | 1 | file func.ts content does not match expected:
-... | Shell |
| E8 | 1 | output contains 'successfully' but should not | Shell |
| E9 | 1 | file numbered.txt content does not match expect... | Shell |
| E11 | 1 | file process.py content does not match expected... | Shell |
| C3 | 1 | file handler1.ts content does not match expecte... | Shell |
| C5 | 1 | output does not contain 'wire.go' | Shell |
| S1 | 2 | output does not contain 'func calculateTotal(it... | Shell |
| S2 | 2 | output does not contain '7' | Shell |
| S4 | 2 | output does not contain 'folder3' | Shell |
| R1 | 2 | output does not contain 'db_host=localhost' | Shell |
| W2 | 2 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 2 | file special.txt content does not match expecte... | Shell |
| E2 | 2 | file file.txt content does not match expected:
... | Shell |
| E3 | 2 | file data.txt content does not match expected:
... | Shell |
| E6 | 2 | file values.ts content does not match expected:... | Shell |
| E8 | 2 | output contains 'successfully' but should not | Shell |
| C3 | 2 | file handler1.ts content does not match expecte... | Shell |
| C5 | 2 | output does not contain 'wire.go' | Shell |
| S1 | 3 | output does not contain 'func calculateTotal(it... | Shell |
| S2 | 3 | output does not contain '7' | Shell |
| S4 | 3 | output does not contain 'folder3' | Shell |
| R1 | 3 | output does not contain 'db_host=localhost' | Shell |
| W2 | 3 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 3 | file special.txt content does not match expecte... | Shell |
| E2 | 3 | file file.txt content does not match expected:
... | Shell |
| E6 | 3 | file values.ts content does not match expected:... | Shell |
| E7 | 3 | file func.ts content does not match expected:
-... | Shell |
| E8 | 3 | output contains 'successfully' but should not | Shell |
| E11 | 3 | file process.py content does not match expected... | Shell |
| C3 | 3 | file handler1.ts content does not match expecte... | Shell |
| C5 | 3 | output does not contain 'wire.go' | Shell |
| S1 | 4 | output does not contain 'func calculateTotal(it... |  |
| S2 | 4 | output does not contain '7' | Shell |
| S4 | 4 | output does not contain 'folder3' | Shell |
| W2 | 4 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 4 | file special.txt content does not match expecte... | Shell |
| E2 | 4 | file file.txt content does not match expected:
... | Shell |
| E3 | 4 | file data.txt content does not match expected:
... | Shell |
| E6 | 4 | file values.ts content does not match expected:... | Shell |
| E8 | 4 | output contains 'successfully' but should not | Shell |
| C3 | 4 | file handler1.ts content does not match expecte... | Shell |
| C5 | 4 | output does not contain 'wire.go' | Shell |
| S1 | 5 | output does not contain 'func calculateTotal(it... | Shell |
| S2 | 5 | output does not contain '7' | Shell |
| S4 | 5 | output does not contain 'folder3' | Shell |
| W2 | 5 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 5 | file special.txt content does not match expecte... | Shell |
| E2 | 5 | file file.txt content does not match expected:
... | Shell |
| E3 | 5 | file data.txt content does not match expected:
... | Shell |
| E7 | 5 | file func.ts content does not match expected:
-... | Shell |
| E8 | 5 | output contains 'successfully' but should not | Shell |
| E9 | 5 | file numbered.txt content does not match expect... | Shell |
| E11 | 5 | file process.py content does not match expected... | Shell |
| C3 | 5 | file handler1.ts content does not match expecte... | Shell |
| C5 | 5 | output does not contain 'wire.go' | Shell |
| S1 | 6 | output does not contain 'func calculateTotal(it... | Shell |
| S2 | 6 | output does not contain '7' | Shell |
| S3 | 6 | output contains 'helpers.go' but should not | Shell |
| S4 | 6 | output does not contain 'folder3' | Shell |
| W2 | 6 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 6 | file special.txt content does not match expecte... | Shell |
| E2 | 6 | file file.txt content does not match expected:
... | Shell |
| E3 | 6 | file data.txt content does not match expected:
... | Shell |
| E6 | 6 | file values.ts content does not match expected:... | Shell |
| E7 | 6 | file func.ts content does not match expected:
-... | Shell |
| E8 | 6 | output contains 'successfully' but should not | Shell |
| E9 | 6 | file numbered.txt content does not match expect... |  |
| C1 | 6 | output does not contain 'db.example.com' | Shell |
| C3 | 6 | file handler1.ts content does not match expecte... | Shell |
| C5 | 6 | output does not contain 'wire.go' | Shell |
| S1 | 7 | output does not contain 'func calculateTotal(it... | Shell |
| S2 | 7 | output does not contain '7' | Shell |
| R1 | 7 | output does not contain 'db_host=localhost' | Shell |
| W2 | 7 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 7 | file special.txt content does not match expecte... | Shell |
| E2 | 7 | file file.txt content does not match expected:
... | Shell |
| E3 | 7 | file data.txt content does not match expected:
... | Shell |
| E6 | 7 | file values.ts content does not match expected:... | Shell |
| E7 | 7 | file func.ts content does not match expected:
-... | Shell |
| E8 | 7 | output contains 'successfully' but should not | Shell |
| E11 | 7 | file process.py content does not match expected... | Shell |
| C1 | 7 | output does not contain 'db.example.com' | Shell |
| C3 | 7 | file handler1.ts content does not match expecte... | Shell |
| C5 | 7 | output does not contain 'wire.go' | Shell |
| S1 | 8 | output does not contain 'func calculateTotal(it... | Shell |
| S2 | 8 | output does not contain '7' | Shell |
| S4 | 8 | output does not contain 'folder3' | Shell |
| R1 | 8 | output does not contain 'db_host=localhost' | Shell |
| W2 | 8 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 8 | file special.txt content does not match expecte... | Shell |
| E2 | 8 | file file.txt content does not match expected:
... | Shell |
| E3 | 8 | file data.txt content does not match expected:
... | Shell |
| E6 | 8 | file values.ts content does not match expected:... | Shell |
| E7 | 8 | file func.ts content does not match expected:
-... | Shell |
| E8 | 8 | output contains 'successfully' but should not | Shell |
| E9 | 8 | file numbered.txt content does not match expect... | Shell |
| C1 | 8 | output does not contain 'db.example.com' | Shell |
| C3 | 8 | file handler1.ts content does not match expecte... | Shell |
| C5 | 8 | output does not contain 'wire.go' | Shell |
| S1 | 9 | output does not contain 'func calculateTotal(it... | Shell |
| S2 | 9 | output does not contain '7' | Shell |
| R1 | 9 | output does not contain 'db_host=localhost' | Shell |
| W2 | 9 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 9 | file special.txt content does not match expecte... | Shell |
| E2 | 9 | file file.txt content does not match expected:
... | Shell |
| E3 | 9 | file data.txt content does not match expected:
... | Shell |
| E6 | 9 | file values.ts content does not match expected:... | Shell |
| E7 | 9 | file func.ts content does not match expected:
-... | Shell |
| E8 | 9 | output contains 'successfully' but should not | Shell |
| E9 | 9 | file numbered.txt content does not match expect... | Shell |
| C1 | 9 | output does not contain 'db.example.com' | Shell |
| C3 | 9 | file handler1.ts content does not match expecte... | Shell |
| C5 | 9 | output does not contain 'wire.go' | Shell |
| S1 | 10 | output does not contain 'func calculateTotal(it... | Shell |
| S2 | 10 | output does not contain '7' | Shell |
| S3 | 10 | output contains 'helpers.go' but should not | Shell |
| W2 | 10 | file main.go does not exist; failed to read fil... | Shell |
| W4 | 10 | file special.txt content does not match expecte... | Shell |
| E2 | 10 | file file.txt content does not match expected:
... | Shell |
| E3 | 10 | file data.txt content does not match expected:
... | Shell |
| E6 | 10 | file values.ts content does not match expected:... | Shell |
| E7 | 10 | file func.ts content does not match expected:
-... | Shell |
| E8 | 10 | output contains 'successfully' but should not | Shell |
| E11 | 10 | file process.py content does not match expected... | Shell |
| C1 | 10 | output does not contain 'db.example.com' | Shell |
| C3 | 10 | file handler1.ts content does not match expecte... | Shell |
| C5 | 10 | output does not contain 'wire.go' | Shell |

## Appendix A: Configuration

### Version

```
kvit-coder baaf1fc (commit 20260103, built 2026-01-03)
```

### config.yaml

```yaml

```

