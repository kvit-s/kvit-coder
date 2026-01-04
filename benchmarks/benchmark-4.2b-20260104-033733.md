# LLM Tool Usage Benchmark Report

## Metadata

- **Version**: kvit-coder 855d738 (commit 20260104, built 2026-01-04)
- **Date**: 2026-01-04T04:43:52-06:00
- **Total Benchmarks**: 28
- **Total Runs**: 280

## Summary

| Class | Success Rate | Avg Time/Run |
|-------|--------------|--------------|
| C | 0% (0/40) | 47.3s |
| E | 8% (9/110) | 199.9s |
| R | 0% (0/30) | 12.5s |
| S | 10% (4/40) | 56.3s |
| W | 3% (2/60) | 62.0s |
| **Total** | **5% (15/280)** | **378.0s** |

## Detailed Statistics

### Per-Benchmark Summary

| Benchmark | Success | LLM Calls | Tokens | Generated | Context | Prompt Speed | Gen Speed | Cost | Duration |
|-----------|---------|-----------|--------|-----------|---------|--------------|-----------|------|----------|
| C1 | 0% | 6.1(±3.1) | 19101(±9254) | 979(±1419) | 3696 | 4011.9 t/s | 265.9 t/s | $0.0000 | 6.7s(±7.4) |
| C2 | 0% | 7.2(±3.7) | 25221(±11874) | 3757(±4297) | 5866 | 14260.8 t/s | 912.9 t/s | $0.0000 | 20.3s(±22.0) |
| C3 | 0% | 6.2(±4.4) | 23426(±19399) | 1552(±1971) | 4553 | 7994.0 t/s | 254.0 t/s | $0.0000 | 9.2s(±8.8) |
| C5 | 0% | 5.9(±2.7) | 19705(±7995) | 1919(±2422) | 4753 | 8836.9 t/s | 393.9 t/s | $0.0000 | 11.2s(±12.5) |
| E1 | 0% | 3.8(±2.0) | 14144(±7961) | 2555(±4975) | 5073 | 5522.3 t/s | 563.8 t/s | $0.0000 | 28.2s(±45.1) |
| E2 | 0% | 2.1(±1.4) | 7170(±4212) | 1070(±1418) | 3873 | 3630.4 t/s | 321.2 t/s | $0.0000 | 5.8s(±6.5) |
| E3 | 0% | 3.3(±1.3) | 10218(±4096) | 509(±543) | 3336 | 2618.8 t/s | 167.9 t/s | $0.0000 | 3.2s(±2.4) |
| E4 | 0% | 4.8(±2.6) | 17232(±10774) | 2441(±2793) | 5292 | 11860.6 t/s | 569.3 t/s | $0.0000 | 14.2s(±16.6) |
| E5 | 0% | 6.1(±4.0) | 22781(±16068) | 1913(±1748) | 4764 | 7973.6 t/s | 301.7 t/s | $0.0000 | 23.1s(±33.6) |
| E6 | 0% | 4.4(±2.7) | 17859(±9824) | 4764(±5307) | 7327 | 10042.4 t/s | 954.0 t/s | $0.0000 | 40.5s(±40.8) |
| E7 | 0% | 3.9(±1.8) | 11895(±5394) | 365(±214) | 3183 | 1348.3 t/s | 135.9 t/s | $0.0000 | 14.5s(±35.2) |
| E8 | 90% | 2.5(±1.0) | 9963(±4660) | 2746(±3126) | 5516 | 7517.5 t/s | 487.3 t/s | $0.0000 | 15.5s(±18.7) |
| E9 | 0% | 2.0(±1.5) | 7502(±5924) | 1718(±2008) | 4408 | 3156.4 t/s | 306.0 t/s | $0.0000 | 8.4s(±9.4) |
| E10 | 0% | 3.6(±2.6) | 13229(±8512) | 2393(±3049) | 4771 | 5549.0 t/s | 438.5 t/s | $0.0000 | 25.8s(±35.8) |
| E11 | 0% | 7.0(±3.9) | 22776(±11925) | 1561(±1708) | 3885 | 2893.5 t/s | 214.1 t/s | $0.0000 | 20.7s(±33.8) |
| R1 | 0% | 3.7(±1.6) | 11143(±4629) | 341(±102) | 3150 | 4208.7 t/s | 138.1 t/s | $0.0000 | 2.8s(±1.1) |
| R2 | 0% | 5.0(±1.7) | 16086(±6647) | 1161(±1959) | 3916 | 8566.6 t/s | 360.3 t/s | $0.0000 | 7.0s(±11.4) |
| R3 | 0% | 3.2(±1.7) | 9669(±4933) | 450(±275) | 3188 | 1252.0 t/s | 178.4 t/s | $0.0000 | 2.7s(±1.2) |
| S1 | 0% | 5.1(±3.4) | 17241(±12269) | 827(±749) | 3756 | 4412.9 t/s | 151.1 t/s | $0.0000 | 5.6s(±4.9) |
| S2 | 40% | 3.9(±2.0) | 13214(±7480) | 759(±536) | 3954 | 4297.4 t/s | 175.1 t/s | $0.0000 | 4.5s(±2.8) |
| S3 | 0% | 5.2(±3.5) | 18022(±9987) | 2665(±5003) | 5199 | 4375.2 t/s | 564.0 t/s | $0.0000 | 39.8s(±52.4) |
| S4 | 0% | 4.4(±3.1) | 28812(±51375) | 940(±1784) | 5966 | 10269.3 t/s | 346.6 t/s | $0.0000 | 6.4s(±9.4) |
| W1 | 10% | 1.5(±0.7) | 4744(±2031) | 450(±242) | 3273 | 1139.3 t/s | 213.1 t/s | $0.0000 | 2.1s(±1.1) |
| W2 | 0% | 1.2(±1.0) | 4507(±3015) | 1065(±1259) | 3614 | 757.0 t/s | 215.6 t/s | $0.0000 | 16.8s(±34.8) |
| W3 | 0% | 2.3(±2.2) | 7281(±7113) | 566(±679) | 2995 | 4010.5 t/s | 185.5 t/s | $0.0000 | 15.1s(±35.1) |
| W4 | 0% | 1.2(±0.4) | 5885(±2659) | 2454(±2840) | 5290 | 16334.6 t/s | 1047.6 t/s | $0.0000 | 21.2s(±35.2) |
| W5 | 10% | 1.9(±1.5) | 6355(±4722) | 924(±904) | 3583 | 299.2 t/s | 214.4 t/s | $0.0000 | 4.3s(±3.9) |
| W6 | 0% | 1.6(±0.9) | 5109(±3070) | 494(±477) | 3302 | 899.3 t/s | 217.4 t/s | $0.0000 | 2.4s(±2.3) |

### Per-Benchmark Details

#### C1

**✗ Search Then Read** | Find and read a file | 0% (0/10) | 6.1 calls | 19101 tokens | 6.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 5.4s | 8 | 24231 | 781 | 3236 | $0.0000 | Search×3 |
| 2 | ✗ FAIL | 2.6s | 4 | 11754 | 211 | 2971 | $0.0000 | Search×2 |
| 3 | ✗ FAIL | 3.7s | 6 | 18097 | 433 | 3058 | $0.0000 | Search×2 |
| 4 | ✗ FAIL | 4.3s | 8 | 24489 | 609 | 3310 | $0.0000 | Search×4 |
| 5 | ✗ FAIL | 3.5s | 6 | 18473 | 331 | 3296 | $0.0000 | Search×5 |
| 6 | ✗ FAIL | 2.9s | 3 | 9473 | 539 | 3427 | $0.0000 | Search×3 |
| 7 | ✗ FAIL | 2.3s | 3 | 8844 | 207 | 3002 | $0.0000 | Search |
| 8 | ✗ FAIL | 28.3s | 2 | 10934 | 5176 | 8037 | $0.0000 | Search |
| 9 | ✗ FAIL | 9.1s | 13 | 40404 | 1020 | 3360 | $0.0000 | Search×6 |
| 10 | ✗ FAIL | 4.8s | 8 | 24313 | 482 | 3259 | $0.0000 | Search×5 |

**Failures:**

**Run 1**: output does not contain 'db.example.com'

**Run 2**: cancelled

**Run 3**: output does not contain 'db.example.com'

**Run 4**: output does not contain 'db.example.com'

**Run 5**: output does not contain 'db.example.com'

**Run 6**: output does not contain 'db.example.com'

**Run 7**: output does not contain 'db.example.com'

**Run 8**: output does not contain 'db.example.com'

**Run 9**: output does not contain 'db.example.com'

**Run 10**: output does not contain 'db.example.com'

---

#### C2

**✗ Read-Modify-Write** | Complete edit workflow | 0% (0/10) | 7.2 calls | 25221 tokens | 20.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 69.3s | 8 | 36053 | 12742 | 15283 | $0.0000 | Search×2 |
| 2 | ✗ FAIL | 4.0s | 6 | 17934 | 357 | 3034 | $0.0000 | Search×3 |
| 3 | ✗ FAIL | 26.8s | 12 | 43120 | 5470 | 8104 | $0.0000 | Search×6 |
| 4 | ✗ FAIL | 7.4s | 14 | 42436 | 1045 | 3178 | $0.0000 | Search×3 |
| 5 | ✗ FAIL | 2.4s | 4 | 11780 | 236 | 2973 | $0.0000 | Search×2 |
| 6 | ✗ FAIL | 34.9s | 5 | 21545 | 7121 | 8033 | $0.0000 | Search |
| 7 | ✗ FAIL | 6.3s | 11 | 35145 | 1014 | 3564 | $0.0000 | Read, Search×4 |
| 8 | ✗ FAIL | 3.6s | 5 | 14759 | 335 | 3019 | $0.0000 | Search×2 |
| 9 | ✗ FAIL | 45.5s | 2 | 14625 | 8961 | 8443 | $0.0000 | - |
| 10 | ✗ FAIL | 2.5s | 5 | 14811 | 287 | 3026 | $0.0000 | Search×2 |

**Failures:**

**Run 1**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+  port: 3000
 

```


**Run 2**: cancelled

**Run 3**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+  port: 3000
 

```


**Run 4**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+  port: 3000
 

```


**Run 5**: cancelled

**Run 6**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+  port: 3000
 

```


**Run 7**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+  port: 3000
 

```


**Run 8**: cancelled

**Run 9**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+  port: 3000
 

```


**Run 10**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+  port: 3000
 

```


---

#### C3

**✗ Search-Read-Edit** | Find, understand, and modify | 0% (0/10) | 6.2 calls | 23426 tokens | 9.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 12.7s | 5 | 18149 | 2252 | 5228 | $0.0000 | Search×3 |
| 2 | ✗ FAIL | 5.9s | 10 | 31751 | 791 | 3637 | $0.0000 | Search×5 |
| 3 | ✗ FAIL | 17.8s | 17 | 75479 | 2548 | 5939 | $0.0000 | Search×13 |
| 4 | ✗ FAIL | 3.4s | 7 | 20660 | 383 | 2984 | $0.0000 | Search×2 |
| 5 | ✗ FAIL | 1.9s | 3 | 8974 | 253 | 3125 | $0.0000 | Search×2 |
| 6 | ✗ FAIL | 2.2s | 3 | 9120 | 379 | 3258 | $0.0000 | Search×2 |
| 7 | ✗ FAIL | 2.5s | 3 | 9066 | 315 | 3136 | $0.0000 | Search×2 |
| 8 | ✗ FAIL | 6.1s | 8 | 34499 | 521 | 5475 | $0.0000 | Search×6 |
| 9 | ✗ FAIL | 7.8s | 4 | 13900 | 1096 | 4325 | $0.0000 | Search×3 |
| 10 | ✗ FAIL | 31.4s | 2 | 12657 | 6987 | 8424 | $0.0000 | - |

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


**Run 4**: cancelled

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

**✗ Needle in Haystack Search** | Find specific initialization among many usages | 0% (0/10) | 5.9 calls | 19705 tokens | 11.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 8.1s | 4 | 14905 | 1744 | 5997 | $0.0000 | Search×2 |
| 2 | ✗ FAIL | 11.8s | 12 | 38545 | 1287 | 3945 | $0.0000 | Search×7 |
| 3 | ✗ FAIL | 4.2s | 8 | 24418 | 540 | 3264 | $0.0000 | Search×4 |
| 4 | ✗ FAIL | 43.9s | 5 | 22473 | 7955 | 9836 | $0.0000 | Search |
| 5 | ✗ FAIL | 5.2s | 4 | 12395 | 471 | 3337 | $0.0000 | Search×3 |
| 6 | ✗ FAIL | 2.4s | 5 | 14771 | 299 | 2984 | $0.0000 | Search |
| 7 | ✗ FAIL | 3.3s | 3 | 9272 | 626 | 3200 | $0.0000 | Search×2 |
| 8 | ✗ FAIL | 3.6s | 8 | 24147 | 514 | 3223 | $0.0000 | Search×3 |
| 9 | ✗ FAIL | 5.7s | 7 | 21185 | 679 | 3258 | $0.0000 | Search×3 |
| 10 | ✗ FAIL | 24.0s | 3 | 14935 | 5077 | 8484 | $0.0000 | Search |

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

**✗ Single Line Replace** | Replace a single line | 0% (0/10) | 3.8 calls | 14144 tokens | 28.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.4s | 4 | 12349 | 403 | 3220 | $0.0000 | Search×2 |
| 2 | ✗ FAIL | 2.7s | 5 | 15275 | 289 | 3106 | $0.0000 | Search×2 |
| 3 | ✗ FAIL | 2.5s | 5 | 15849 | 453 | 3439 | $0.0000 | Search×3 |
| 4 | ✗ FAIL | 4.5s | 4 | 12695 | 749 | 3566 | $0.0000 | Edit, Search×2 |
| 5 | ✗ FAIL | 115.9s | 4 | 28742 | 17094 | 19759 | $0.0000 | Search×2 |
| 6 | ✗ FAIL | 19.5s | 3 | 14821 | 4156 | 6872 | $0.0000 | Search×2 |
| 7 | ✗ FAIL | 4.2s | 8 | 25541 | 738 | 3476 | $0.0000 | Search×4 |
| 8 | ✗ FAIL | 2.2s | 2 | 6163 | 414 | 3251 | $0.0000 | Search |
| 9 | ✗ FAIL | 8.4s | 3 | 10004 | 1256 | 4045 | $0.0000 | Search, Write |
| 10 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |

**Failures:**

**Run 1**: cancelled

**Run 2**: cancelled

**Run 3**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 FUNCTION B
 FUNCTION C
 FUNCTION D
-NEW LINE E
+FUNCTION E
 FUNCTION F
 FUNCTION G
 FUNCTION H

```


**Run 4**: cancelled

**Run 5**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 FUNCTION B
 FUNCTION C
 FUNCTION D
-NEW LINE E
+FUNCTION E
 FUNCTION F
 FUNCTION G
 FUNCTION H

```


**Run 6**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 FUNCTION B
 FUNCTION C
 FUNCTION D
-NEW LINE E
+FUNCTION E
 FUNCTION F
 FUNCTION G
 FUNCTION H

```


**Run 7**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 FUNCTION B
 FUNCTION C
 FUNCTION D
-NEW LINE E
+FUNCTION E
 FUNCTION F
 FUNCTION G
 FUNCTION H

```


**Run 8**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 FUNCTION B
 FUNCTION C
 FUNCTION D
-NEW LINE E
+FUNCTION E
 FUNCTION F
 FUNCTION G
 FUNCTION H

```


**Run 9**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 FUNCTION B
 FUNCTION C
 FUNCTION D
-NEW LINE E
+FUNCTION E
 FUNCTION F
 FUNCTION G
 FUNCTION H

```


**Run 10**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,7 @@
 FUNCTION B
 FUNCTION C
 FUNCTION D
-NEW LINE E
+FUNCTION E
 FUNCTION F
 FUNCTION G
 FUNCTION H

```


---

#### E2

**✗ Multi-line Insert** | Insert multiple lines at position | 0% (0/10) | 2.1 calls | 7170 tokens | 5.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.9s | 1 | 3309 | 468 | 3309 | $0.0000 | - |
| 2 | ✗ FAIL | 6.0s | 1 | 4068 | 1227 | 4068 | $0.0000 | - |
| 3 | ✗ FAIL | 1.3s | 2 | 5930 | 175 | 2965 | $0.0000 | Search |
| 4 | ✗ FAIL | 1.3s | 2 | 5920 | 165 | 3005 | $0.0000 | Search |
| 5 | ✗ FAIL | 1.9s | 2 | 6008 | 260 | 3079 | $0.0000 | Search |
| 6 | ✗ FAIL | 23.8s | 1 | 7912 | 5071 | 7912 | $0.0000 | - |
| 7 | ✗ FAIL | 0.9s | 2 | 5855 | 100 | 2948 | $0.0000 | Search |
| 8 | ✗ FAIL | 6.3s | 6 | 18688 | 586 | 3306 | $0.0000 | Search×4 |
| 9 | ✗ FAIL | 6.1s | 3 | 9596 | 1073 | 3729 | $0.0000 | Search |
| 10 | ✗ FAIL | 8.0s | 1 | 4412 | 1571 | 4412 | $0.0000 | - |

**Failures:**

**Run 1**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 2**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 3**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 4**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 5**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 6**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 7**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 8**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 9**: cancelled

**Run 10**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,6 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


---

#### E3

**✗ Delete Lines** | Delete a range of lines | 0% (0/10) | 3.3 calls | 10218 tokens | 3.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.9s | 2 | 6106 | 355 | 3141 | $0.0000 | Search |
| 2 | ✗ FAIL | 1.5s | 2 | 6083 | 205 | 3187 | $0.0000 | Search |
| 3 | ✗ FAIL | 9.9s | 4 | 13675 | 2100 | 4789 | $0.0000 | Search |
| 4 | ✗ FAIL | 3.9s | 6 | 18032 | 416 | 3130 | $0.0000 | Search×3 |
| 5 | ✗ FAIL | 2.5s | 1 | 3448 | 609 | 3448 | $0.0000 | - |
| 6 | ✗ FAIL | 2.8s | 4 | 12268 | 306 | 3152 | $0.0000 | Search |
| 7 | ✗ FAIL | 2.1s | 4 | 12223 | 282 | 3099 | $0.0000 | Search×2 |
| 8 | ✗ FAIL | 1.1s | 3 | 8825 | 162 | 2974 | $0.0000 | Search |
| 9 | ✗ FAIL | 3.7s | 4 | 12197 | 259 | 3130 | $0.0000 | Search×2 |
| 10 | ✗ FAIL | 3.0s | 3 | 9321 | 400 | 3305 | $0.0000 | Search |

**Failures:**

**Run 1**: 
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


**Run 3**: 
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


**Run 7**: cancelled

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


**Run 9**: cancelled

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

**✗ Boundary Edit** | Test boundary conditions | 0% (0/10) | 4.8 calls | 17232 tokens | 14.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.9s | 2 | 6524 | 655 | 3591 | $0.0000 | Search |
| 2 | ✗ FAIL | 48.1s | 8 | 32116 | 7858 | 9707 | $0.0000 | Search×3 |
| 3 | ✗ FAIL | 13.2s | 4 | 14286 | 2748 | 5315 | $0.0000 | Search |
| 4 | ✗ FAIL | 3.0s | 6 | 18381 | 376 | 3150 | $0.0000 | Search×2 |
| 5 | ✗ FAIL | 1.1s | 2 | 5888 | 145 | 2990 | $0.0000 | Search |
| 6 | ✗ FAIL | 40.4s | 4 | 18681 | 6991 | 9700 | $0.0000 | Search×2 |
| 7 | ✗ FAIL | 2.5s | 6 | 17765 | 425 | 3044 | $0.0000 | Search |
| 8 | ✗ FAIL | 3.1s | 4 | 12017 | 410 | 3177 | $0.0000 | Search×2 |
| 9 | ✗ FAIL | 3.3s | 2 | 6271 | 535 | 3361 | $0.0000 | Search |
| 10 | ✗ FAIL | 24.3s | 10 | 40390 | 4270 | 8883 | $0.0000 | Read, Search×6 |

**Failures:**

**Run 1**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 2**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 3**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 4**: cancelled

**Run 5**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 6**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 7**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 8**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 9**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 10**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


---

#### E5

**✗ Replace All Occurrences** | Replace multiple occurrences | 0% (0/10) | 6.1 calls | 22781 tokens | 23.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.3s | 2 | 5975 | 226 | 3095 | $0.0000 | Search |
| 2 | ✗ FAIL | 6.0s | 4 | 12849 | 915 | 3494 | $0.0000 | Search×2 |
| 3 | ✗ FAIL | 18.4s | 15 | 55353 | 2800 | 5839 | $0.0000 | Read×2, Search×8 |
| 4 | ✗ FAIL | 18.9s | 2 | 9051 | 3310 | 6123 | $0.0000 | Search |
| 5 | ✗ FAIL | 1.4s | 4 | 12959 | 230 | 3361 | $0.0000 | Search |
| 6 | ✗ FAIL | 5.1s | 2 | 6498 | 749 | 3595 | $0.0000 | Search |
| 7 | ✗ FAIL | 120.0s | 9 | 31889 | 710 | 4025 | $0.0000 | Search×6 |
| 8 | ✗ FAIL | 16.2s | 10 | 44529 | 2999 | 5260 | $0.0000 | Search×10 |
| 9 | ✗ FAIL | 33.2s | 7 | 28155 | 5994 | 8479 | $0.0000 | Search×2 |
| 10 | ✗ FAIL | 10.4s | 6 | 20549 | 1200 | 4364 | $0.0000 | Search×5 |

**Failures:**

**Run 1**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


**Run 2**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


**Run 3**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


**Run 4**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


**Run 5**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


**Run 6**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


**Run 7**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


**Run 8**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


**Run 9**: cancelled

**Run 10**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
 function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+    const result = oldFunc();
+    if (oldFunc() !== null) {
         return;
     }
-    const value = newFunc();
+    const value = oldFunc();
 }
 
-function newFunc(): string | null {
+function oldFunc(): string | null {
     return null;
 }
 

```


---

#### E6

**✗ Context-Specific Replace** | Replace with context for uniqueness | 0% (0/10) | 4.4 calls | 17859 tokens | 40.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 6.0s | 2 | 7132 | 1391 | 4223 | $0.0000 | Search |
| 2 | ✗ FAIL | 7.7s | 4 | 12899 | 1344 | 4070 | $0.0000 | Search |
| 3 | ✗ FAIL | 3.2s | 2 | 6365 | 625 | 3472 | $0.0000 | Search |
| 4 | ✗ FAIL | 47.8s | 11 | 41676 | 7437 | 9368 | $0.0000 | Search×7 |
| 5 | ✗ FAIL | 120.0s | 6 | 17741 | 377 | 3001 | $0.0000 | Search |
| 6 | ✗ FAIL | 10.6s | 6 | 20694 | 2290 | 5142 | $0.0000 | Search×4 |
| 7 | ✗ FAIL | 1.9s | 3 | 8816 | 170 | 2964 | $0.0000 | Search |
| 8 | ✗ FAIL | 103.5s | 1 | 20397 | 17563 | 20397 | $0.0000 | - |
| 9 | ✗ FAIL | 51.2s | 4 | 19968 | 8002 | 10897 | $0.0000 | Search |
| 10 | ✗ FAIL | 52.8s | 5 | 22898 | 8440 | 9736 | $0.0000 | Search |

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


**Run 4**: cancelled

**Run 5**: 
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

**✗ Multi-line Block Replace** | Replace multi-line block | 0% (0/10) | 3.9 calls | 11895 tokens | 14.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 3.4s | 1 | 3687 | 851 | 3687 | $0.0000 | - |
| 2 | ✗ FAIL | 1.1s | 2 | 5815 | 89 | 2914 | $0.0000 | Search |
| 3 | ✗ FAIL | 2.6s | 4 | 12210 | 242 | 3128 | $0.0000 | Search×2 |
| 4 | ✗ FAIL | 120.0s | 6 | 17862 | 414 | 3059 | $0.0000 | Search×3 |
| 5 | ✗ FAIL | 2.8s | 5 | 14725 | 257 | 2976 | $0.0000 | Search×2 |
| 6 | ✗ FAIL | 2.1s | 2 | 5966 | 222 | 3023 | $0.0000 | Search |
| 7 | ✗ FAIL | 3.0s | 4 | 11976 | 416 | 3169 | $0.0000 | Search |
| 8 | ✗ FAIL | 4.9s | 7 | 22262 | 630 | 3641 | $0.0000 | Search×4 |
| 9 | ✗ FAIL | 2.1s | 4 | 12179 | 229 | 3097 | $0.0000 | Search×2 |
| 10 | ✗ FAIL | 2.7s | 4 | 12265 | 297 | 3140 | $0.0000 | Search |

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


**Run 2**: 
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


**Run 3**: cancelled

**Run 4**: 
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


**Run 5**: cancelled

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


**Run 9**: cancelled

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

**✗ No Match Handling** | Handle search text not found | 90% (9/10) | 2.5 calls | 9963 tokens | 15.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.3s | 1 | 3169 | 335 | 3169 | $0.0000 | - |
| 2 | ✓ PASS | 2.4s | 3 | 8894 | 244 | 2986 | $0.0000 | Search |
| 3 | ✓ PASS | 1.7s | 3 | 9129 | 297 | 3122 | $0.0000 | Search×2 |
| 4 | ✓ PASS | 45.1s | 4 | 19038 | 7504 | 10188 | $0.0000 | Search |
| 5 | ✓ PASS | 13.5s | 3 | 11261 | 2613 | 5329 | $0.0000 | Search |
| 6 | ✓ PASS | 2.0s | 1 | 3278 | 444 | 3278 | $0.0000 | - |
| 7 | ✗ FAIL | 16.4s | 2 | 9347 | 3606 | 6391 | $0.0000 | Search |
| 8 | ✓ PASS | 10.2s | 2 | 7723 | 1982 | 4802 | $0.0000 | Search |
| 9 | ✓ PASS | 57.4s | 2 | 15329 | 9587 | 12413 | $0.0000 | Search |
| 10 | ✓ PASS | 5.3s | 4 | 12466 | 852 | 3484 | $0.0000 | Search×2 |

**Failures:**

**Run 7**: output contains 'replaced' but should not

---

#### E9

**✗ Empty Content** | Handle empty replacement | 0% (0/10) | 2.0 calls | 7502 tokens | 8.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 18.0s | 6 | 21658 | 3854 | 6319 | $0.0000 | Search×3 |
| 2 | ✗ FAIL | 4.5s | 1 | 3822 | 980 | 3822 | $0.0000 | - |
| 3 | ✗ FAIL | 14.6s | 2 | 8915 | 3164 | 5979 | $0.0000 | Search |
| 4 | ✗ FAIL | 3.2s | 2 | 6297 | 613 | 3190 | $0.0000 | - |
| 5 | ✗ FAIL | 3.1s | 2 | 6251 | 493 | 3359 | $0.0000 | Search |
| 6 | ✗ FAIL | 1.0s | 1 | 3054 | 212 | 3054 | $0.0000 | - |
| 7 | ✗ FAIL | 1.0s | 1 | 3036 | 194 | 3036 | $0.0000 | - |
| 8 | ✗ FAIL | 2.6s | 1 | 3360 | 518 | 3360 | $0.0000 | - |
| 9 | ✗ FAIL | 4.7s | 1 | 3461 | 619 | 3461 | $0.0000 | - |
| 10 | ✗ FAIL | 31.3s | 3 | 15170 | 6530 | 8502 | $0.0000 | Search |

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
+LINE E
 LINE F
 LINE G
 LINE H

```


**Run 2**: 
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


**Run 3**: 
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


**Run 4**: 
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
+LINE E
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


**Run 7**: 
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
+LINE E
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


**Run 10**: 
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

**✗ Special Characters** | Handle special characters in replacement | 0% (0/10) | 3.6 calls | 13229 tokens | 25.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 2 | ✗ FAIL | 4.2s | 7 | 22223 | 534 | 3482 | $0.0000 | Search×3 |
| 3 | ✗ FAIL | 31.5s | 7 | 27740 | 5684 | 8100 | $0.0000 | Search×4, Write |
| 4 | ✗ FAIL | 5.7s | 3 | 9485 | 734 | 3549 | $0.0000 | Search×2 |
| 5 | ✗ FAIL | 3.1s | 3 | 9099 | 378 | 3069 | $0.0000 | Search×2 |
| 6 | ✗ FAIL | 7.4s | 1 | 4568 | 1729 | 4568 | $0.0000 | - |
| 7 | ✗ FAIL | 5.1s | 7 | 21531 | 825 | 3443 | $0.0000 | Search×3 |
| 8 | ✗ FAIL | 11.7s | 1 | 5172 | 2333 | 5172 | $0.0000 | - |
| 9 | ✗ FAIL | 60.8s | 2 | 16041 | 10286 | 12618 | $0.0000 | Write |
| 10 | ✗ FAIL | 8.1s | 5 | 16433 | 1430 | 3709 | $0.0000 | Search×3, Write |

**Failures:**

**Run 1**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 2**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 3**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 4**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 5**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 6**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 7**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 8**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 9**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


**Run 10**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,6 @@
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


---

#### E11

**✗ Indentation Preservation** | Maintain correct indentation (critical for Python) | 0% (0/10) | 7.0 calls | 22776 tokens | 20.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.7s | 5 | 14772 | 234 | 2974 | $0.0000 | Search×2 |
| 2 | ✗ FAIL | 120.0s | 7 | 21343 | 444 | 3177 | $0.0000 | Search×3 |
| 3 | ✗ FAIL | 4.6s | 9 | 27861 | 435 | 3177 | $0.0000 | Search×2 |
| 4 | ✗ FAIL | 2.7s | 5 | 14839 | 301 | 2999 | $0.0000 | Search×2 |
| 5 | ✗ FAIL | 11.8s | 13 | 41294 | 1761 | 4075 | $0.0000 | Search×3, Write |
| 6 | ✗ FAIL | 6.5s | 14 | 46545 | 1012 | 3859 | $0.0000 | Read, Search×3 |
| 7 | ✗ FAIL | 26.8s | 2 | 11705 | 6005 | 6177 | $0.0000 | - |
| 8 | ✗ FAIL | 13.6s | 5 | 17202 | 2414 | 5123 | $0.0000 | Search×2 |
| 9 | ✗ FAIL | 14.5s | 2 | 8467 | 2619 | 4284 | $0.0000 | Search |
| 10 | ✗ FAIL | 4.1s | 8 | 23728 | 386 | 3004 | $0.0000 | Search×2 |

**Failures:**

**Run 1**: cancelled

**Run 2**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,6 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
 

```


**Run 3**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,6 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
 

```


**Run 4**: cancelled

**Run 5**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,6 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
 

```


**Run 6**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,6 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
 

```


**Run 7**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,6 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
 

```


**Run 8**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,6 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
 

```


**Run 9**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,6 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
 

```


**Run 10**: cancelled

---

#### R1

**✗ Simple File Read** | Read an entire small file | 0% (0/10) | 3.7 calls | 11143 tokens | 2.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.8s | 3 | 9175 | 455 | 3319 | $0.0000 | Search×2 |
| 2 | ✗ FAIL | 3.0s | 2 | 6169 | 448 | 3265 | $0.0000 | Search |
| 3 | ✗ FAIL | 5.1s | 5 | 15270 | 490 | 3234 | $0.0000 | Search, Write |
| 4 | ✗ FAIL | 2.8s | 4 | 11858 | 254 | 3089 | $0.0000 | Search×2 |
| 5 | ✗ FAIL | 0.6s | 1 | 2976 | 148 | 2976 | $0.0000 | - |
| 6 | ✗ FAIL | 2.0s | 4 | 12054 | 247 | 3196 | $0.0000 | Search×3 |
| 7 | ✗ FAIL | 3.2s | 6 | 17924 | 343 | 3047 | $0.0000 | Search×2 |
| 8 | ✗ FAIL | 2.3s | 2 | 6098 | 385 | 3172 | $0.0000 | Search |
| 9 | ✗ FAIL | 3.1s | 5 | 14768 | 336 | 3006 | $0.0000 | Search |
| 10 | ✗ FAIL | 3.2s | 5 | 15142 | 306 | 3198 | $0.0000 | Search×4 |

**Failures:**

**Run 1**: output does not contain 'db_host=localhost'

**Run 2**: output does not contain 'db_host=localhost'

**Run 3**: output does not contain 'db_host=localhost'

**Run 4**: output does not contain 'db_host=localhost'

**Run 5**: output does not contain 'db_host=localhost'

**Run 6**: output does not contain 'db_host=localhost'

**Run 7**: output does not contain 'db_host=localhost'

**Run 8**: output does not contain 'db_host=localhost'

**Run 9**: output does not contain 'db_host=localhost'

**Run 10**: output does not contain 'db_host=localhost'

---

#### R2

**✗ Truncation Recovery** | Handle truncated output by chunked reading | 0% (0/10) | 5.0 calls | 16086 tokens | 7.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 4.1s | 7 | 21520 | 728 | 3257 | $0.0000 | Search×4 |
| 2 | ✗ FAIL | 2.9s | 4 | 11982 | 367 | 3083 | $0.0000 | Search×2 |
| 3 | ✗ FAIL | 2.9s | 6 | 17978 | 373 | 3046 | $0.0000 | Search×3 |
| 4 | ✗ FAIL | 1.1s | 2 | 6322 | 128 | 3436 | $0.0000 | Search |
| 5 | ✗ FAIL | 1.9s | 5 | 14956 | 317 | 3046 | $0.0000 | Search×2 |
| 6 | ✗ FAIL | 2.5s | 4 | 12059 | 347 | 3096 | $0.0000 | Search×2 |
| 7 | ✗ FAIL | 9.0s | 5 | 18161 | 1829 | 4652 | $0.0000 | Search×5 |
| 8 | ✗ FAIL | 3.1s | 6 | 18089 | 416 | 3081 | $0.0000 | Search×2 |
| 9 | ✗ FAIL | 40.7s | 8 | 30890 | 6874 | 9431 | $0.0000 | Search×4 |
| 10 | ✗ FAIL | 1.6s | 3 | 8903 | 235 | 3029 | $0.0000 | Search |

**Failures:**

**Run 1**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 2**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 3**: cancelled

**Run 4**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 5**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 6**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 7**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 8**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 9**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 10**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

---

#### R3

**✗ Relative vs Absolute Path** | Correct path resolution | 0% (0/10) | 3.2 calls | 9669 tokens | 2.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 4.4s | 3 | 9549 | 980 | 3543 | $0.0000 | Search |
| 2 | ✗ FAIL | 1.4s | 1 | 3164 | 334 | 3164 | $0.0000 | - |
| 3 | ✗ FAIL | 1.3s | 2 | 5925 | 190 | 2984 | $0.0000 | Search |
| 4 | ✗ FAIL | 2.8s | 4 | 11759 | 220 | 2959 | $0.0000 | Search×2 |
| 5 | ✗ FAIL | 2.6s | 5 | 14766 | 352 | 2985 | $0.0000 | Search |
| 6 | ✗ FAIL | 1.6s | 2 | 5920 | 194 | 2996 | $0.0000 | Search |
| 7 | ✗ FAIL | 4.9s | 7 | 21133 | 860 | 3462 | $0.0000 | Search×2 |
| 8 | ✗ FAIL | 3.0s | 2 | 6457 | 703 | 3509 | $0.0000 | Search |
| 9 | ✗ FAIL | 2.0s | 3 | 8921 | 285 | 3020 | $0.0000 | Search |
| 10 | ✗ FAIL | 2.5s | 3 | 9095 | 383 | 3263 | $0.0000 | Search×2 |

**Failures:**

**Run 1**: output does not contain 'SUBDIR_CONTENT'

**Run 2**: output does not contain 'SUBDIR_CONTENT'

**Run 3**: output does not contain 'SUBDIR_CONTENT'

**Run 4**: cancelled

**Run 5**: output does not contain 'SUBDIR_CONTENT'

**Run 6**: output does not contain 'SUBDIR_CONTENT'

**Run 7**: output does not contain 'SUBDIR_CONTENT'

**Run 8**: output does not contain 'SUBDIR_CONTENT'

**Run 9**: output does not contain 'SUBDIR_CONTENT'

**Run 10**: output does not contain 'SUBDIR_CONTENT'

---

#### S1

**✗ Simple Pattern Search** | Find a specific function definition | 0% (0/10) | 5.1 calls | 17241 tokens | 5.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 7.4s | 3 | 10585 | 1656 | 4519 | $0.0000 | Search |
| 2 | ✗ FAIL | 1.6s | 3 | 9067 | 138 | 3096 | $0.0000 | Search |
| 3 | ✗ FAIL | 17.2s | 8 | 29575 | 2601 | 5768 | $0.0000 | Read, Search×5 |
| 4 | ✗ FAIL | 10.6s | 12 | 41677 | 1111 | 3865 | $0.0000 | Read, Search×6 |
| 5 | ✗ FAIL | 4.8s | 5 | 16941 | 718 | 3935 | $0.0000 | Search×3 |
| 6 | ✗ FAIL | 2.7s | 3 | 9765 | 578 | 3300 | $0.0000 | Search×6 |
| 7 | ✗ FAIL | 7.6s | 10 | 33691 | 828 | 3738 | $0.0000 | Read×2, Search×4 |
| 8 | ✗ FAIL | 2.4s | 3 | 9028 | 249 | 3143 | $0.0000 | Search×2 |
| 9 | ✗ FAIL | 1.0s | 2 | 6027 | 138 | 3145 | $0.0000 | Search |
| 10 | ✗ FAIL | 1.2s | 2 | 6058 | 249 | 3048 | $0.0000 | Search |

**Failures:**

**Run 1**: output does not contain 'func calculateTotal(items []int)'

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

**✗ Multi-Pattern Search** | Find multiple related items | 40% (4/10) | 3.9 calls | 13214 tokens | 4.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.3s | 2 | 6805 | 467 | 3929 | $0.0000 | Search |
| 2 | ✗ FAIL | 2.3s | 4 | 11930 | 390 | 3081 | $0.0000 | Search |
| 3 | ✓ PASS | 11.3s | 2 | 8441 | 2113 | 5572 | $0.0000 | Search |
| 4 | ✗ FAIL | 3.2s | 3 | 9187 | 550 | 3344 | $0.0000 | Search |
| 5 | ✗ FAIL | 3.1s | 6 | 17955 | 578 | 3110 | $0.0000 | Search |
| 6 | ✗ FAIL | 6.2s | 8 | 31483 | 517 | 5235 | $0.0000 | Read×2, Search×3 |
| 7 | ✗ FAIL | 5.0s | 5 | 15464 | 990 | 3689 | $0.0000 | Search×2 |
| 8 | ✗ FAIL | 1.5s | 2 | 5874 | 140 | 2980 | $0.0000 | Search |
| 9 | ✓ PASS | 6.3s | 5 | 18052 | 1233 | 4528 | $0.0000 | Search |
| 10 | ✓ PASS | 3.3s | 2 | 6948 | 613 | 4076 | $0.0000 | Search |

**Failures:**

**Run 2**: output does not contain '7'

**Run 4**: output does not contain '7'

**Run 5**: output does not contain '7'

**Run 6**: output does not contain '7'

**Run 7**: output does not contain '7'

**Run 8**: output does not contain '7'

---

#### S3

**✗ Search with File Filtering** | Search in specific file types | 0% (0/10) | 5.2 calls | 18022 tokens | 39.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 7.7s | 10 | 31477 | 1530 | 3689 | $0.0000 | Search×4 |
| 2 | ✗ FAIL | 3.8s | 6 | 18257 | 734 | 3277 | $0.0000 | Search×2 |
| 3 | ✗ FAIL | 7.9s | 10 | 30377 | 1167 | 3302 | $0.0000 | Search×2 |
| 4 | ✗ FAIL | 120.0s | 2 | 5958 | 199 | 2990 | $0.0000 | Search |
| 5 | ✗ FAIL | 1.1s | 1 | 3111 | 264 | 3111 | $0.0000 | - |
| 6 | ✗ FAIL | 5.6s | 5 | 15625 | 1102 | 3642 | $0.0000 | Search×2 |
| 7 | ✗ FAIL | 120.0s | 2 | 7726 | 2032 | 4754 | $0.0000 | Search |
| 8 | ✗ FAIL | 3.2s | 5 | 16044 | 357 | 3320 | $0.0000 | Search×2 |
| 9 | ✗ FAIL | 8.9s | 10 | 31231 | 1697 | 3491 | $0.0000 | Search×3 |
| 10 | ✗ FAIL | 119.4s | 1 | 20415 | 17568 | 20415 | $0.0000 | - |

**Failures:**

**Run 1**: output does not contain 'models.go'

**Run 2**: output does not contain 'models.go'

**Run 3**: output does not contain 'models.go'

**Run 4**: output does not contain 'models.go'

**Run 5**: output does not contain 'models.go'

**Run 6**: output does not contain 'models.go'

**Run 7**: output does not contain 'models.go'

**Run 8**: cancelled

**Run 9**: output does not contain 'models.go'

**Run 10**: output does not contain 'models.go'

---

#### S4

**✗ Search Truncation Recovery** | Handle large search results requiring refinement | 0% (0/10) | 4.4 calls | 28812 tokens | 6.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 4.2s | 5 | 14892 | 367 | 3043 | $0.0000 | Search×2 |
| 2 | ✗ FAIL | 1.9s | 4 | 11949 | 261 | 3061 | $0.0000 | Search×2 |
| 3 | ✗ FAIL | 33.7s | 5 | 26683 | 6271 | 10290 | $0.0000 | Search |
| 4 | ✗ FAIL | 2.9s | 2 | 7556 | 327 | 4662 | $0.0000 | Search |
| 5 | ✗ FAIL | 3.0s | 4 | 12057 | 486 | 3128 | $0.0000 | Search×2 |
| 6 | ✗ FAIL | 2.5s | 4 | 11780 | 236 | 2975 | $0.0000 | Search×2 |
| 7 | ✗ FAIL | 1.7s | 3 | 8874 | 234 | 2996 | $0.0000 | Search |
| 8 | ✗ FAIL | 0.9s | 2 | 5850 | 114 | 2971 | $0.0000 | Search |
| 9 | ✗ FAIL | 9.3s | 13 | 181997 | 735 | 23216 | $0.0000 | Search×9 |
| 10 | ✗ FAIL | 4.2s | 2 | 6485 | 368 | 3323 | $0.0000 | Search×2 |

**Failures:**

**Run 1**: output does not contain 'folder3'

**Run 2**: output does not contain 'folder3'

**Run 3**: output does not contain 'folder3'

**Run 4**: output does not contain 'folder3'

**Run 5**: output does not contain 'folder3'

**Run 6**: cancelled

**Run 7**: output does not contain 'folder3'

**Run 8**: output does not contain 'folder3'

**Run 9**: output does not contain 'folder3'

**Run 10**: output does not contain 'folder3'

---

#### W1

**✗ Simple File Creation** | Create a new file with content | 10% (1/10) | 1.5 calls | 4744 tokens | 2.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.2s | 2 | 6204 | 464 | 3212 | $0.0000 | Search |
| 2 | ✗ FAIL | 1.0s | 1 | 3075 | 241 | 3075 | $0.0000 | - |
| 3 | ✗ FAIL | 3.3s | 2 | 6441 | 708 | 3551 | $0.0000 | Search |
| 4 | ✗ FAIL | 1.9s | 1 | 3232 | 398 | 3232 | $0.0000 | - |
| 5 | ✗ FAIL | 0.9s | 1 | 3052 | 218 | 3052 | $0.0000 | - |
| 6 | ✓ PASS | 2.8s | 3 | 9167 | 439 | 3254 | $0.0000 | Search, Write |
| 7 | ✗ FAIL | 2.8s | 2 | 6277 | 548 | 3370 | $0.0000 | Write.cancel |
| 8 | ✗ FAIL | 1.1s | 1 | 3088 | 254 | 3088 | $0.0000 | - |
| 9 | ✗ FAIL | 1.0s | 1 | 3054 | 220 | 3054 | $0.0000 | - |
| 10 | ✗ FAIL | 4.5s | 1 | 3846 | 1012 | 3846 | $0.0000 | - |

**Failures:**

**Run 1**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

**Run 2**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

**Run 3**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

**Run 4**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

**Run 5**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

**Run 7**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

**Run 8**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

**Run 9**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

**Run 10**: file hello.txt does not exist
failed to read file hello.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/hello.txt: no such file or directory

---

#### W2

**✗ Multi-line Content** | Write file with multiple lines and formatting | 0% (0/10) | 1.2 calls | 4507 tokens | 16.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.3s | 1 | 3171 | 333 | 3171 | $0.0000 | - |
| 2 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 3 | ✗ FAIL | 1.2s | 1 | 3094 | 256 | 3094 | $0.0000 | - |
| 4 | ✗ FAIL | 3.8s | 1 | 3647 | 809 | 3647 | $0.0000 | - |
| 5 | ✗ FAIL | 16.2s | 1 | 6653 | 3815 | 6653 | $0.0000 | - |
| 6 | ✗ FAIL | 3.0s | 1 | 3537 | 699 | 3537 | $0.0000 | - |
| 7 | ✗ FAIL | 1.2s | 1 | 3122 | 284 | 3122 | $0.0000 | - |
| 8 | ✗ FAIL | 4.1s | 1 | 3840 | 1002 | 3840 | $0.0000 | - |
| 9 | ✗ FAIL | 14.8s | 1 | 6029 | 3191 | 6029 | $0.0000 | - |
| 10 | ✗ FAIL | 2.4s | 4 | 11979 | 264 | 3049 | $0.0000 | Search×2 |

**Failures:**

**Run 1**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 2**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 3**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 4**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 5**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 6**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 7**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 8**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 9**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


**Run 10**: file main.go does not exist
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
failed to read file main.go: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/main.go: no such file or directory
command failed: exit status 1
output: stat main.go: no such file or directory


---

#### W3

**✗ Overwrite Existing** | Handle overwrite of existing file | 0% (0/10) | 2.3 calls | 7281 tokens | 15.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.0s | 2 | 5873 | 128 | 2967 | $0.0000 | Search |
| 2 | ✗ FAIL | 2.2s | 1 | 3187 | 354 | 3187 | $0.0000 | - |
| 3 | ✗ FAIL | 0.6s | 1 | 2957 | 124 | 2957 | $0.0000 | - |
| 4 | ✗ FAIL | 0.6s | 1 | 2919 | 86 | 2919 | $0.0000 | - |
| 5 | ✗ FAIL | 10.6s | 8 | 25877 | 2019 | 3752 | $0.0000 | Search×3 |
| 6 | ✗ FAIL | 0.8s | 1 | 2980 | 147 | 2980 | $0.0000 | - |
| 7 | ✗ FAIL | 9.2s | 3 | 10464 | 1647 | 4396 | $0.0000 | Search, Write |
| 8 | ✗ FAIL | 2.1s | 4 | 11933 | 283 | 3094 | $0.0000 | Search×2 |
| 9 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 10 | ✗ FAIL | 4.3s | 2 | 6619 | 873 | 3702 | $0.0000 | Search |

**Failures:**

**Run 1**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 2**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 3**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 4**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 5**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 6**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 7**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 8**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 9**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


**Run 10**: 
```diff
file existing.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-new content
-
+old content

```


---

#### W4

**✗ Special Characters** | Handle special characters in content | 0% (0/10) | 1.2 calls | 5885 tokens | 21.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 7.9s | 1 | 4760 | 1916 | 4760 | $0.0000 | - |
| 2 | ✗ FAIL | 4.8s | 2 | 6355 | 573 | 3394 | $0.0000 | Search |
| 3 | ✗ FAIL | 2.4s | 1 | 3419 | 575 | 3419 | $0.0000 | - |
| 4 | ✗ FAIL | 3.1s | 1 | 3495 | 651 | 3495 | $0.0000 | - |
| 5 | ✗ FAIL | 120.0s | 1 | 8624 | 5780 | 8624 | $0.0000 | Write |
| 6 | ✗ FAIL | 1.5s | 1 | 3155 | 311 | 3155 | $0.0000 | - |
| 7 | ✗ FAIL | 37.6s | 1 | 10837 | 7993 | 10837 | $0.0000 | - |
| 8 | ✗ FAIL | 1.9s | 2 | 6066 | 289 | 3080 | $0.0000 | Write |
| 9 | ✗ FAIL | 1.7s | 1 | 3081 | 237 | 3081 | $0.0000 | - |
| 10 | ✗ FAIL | 30.8s | 1 | 9057 | 6213 | 9057 | $0.0000 | - |

**Failures:**

**Run 1**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

**Run 2**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

**Run 3**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

**Run 4**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

**Run 5**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

**Run 6**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

**Run 7**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

**Run 8**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-'quotes', "double", `backticks`, $var
-
+'quotes', "double", "backticks", $var

```


**Run 9**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

**Run 10**: failed to read file special.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/special.txt: no such file or directory

---

#### W5

**✗ Empty File Creation** | Create an empty file | 10% (1/10) | 1.9 calls | 6355 tokens | 4.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 0.7s | 1 | 2989 | 160 | 2989 | $0.0000 | - |
| 2 | ✗ FAIL | 1.4s | 1 | 3112 | 283 | 3112 | $0.0000 | - |
| 3 | ✗ FAIL | 11.1s | 1 | 5381 | 2552 | 5381 | $0.0000 | - |
| 4 | ✗ FAIL | 1.1s | 1 | 3034 | 205 | 3034 | $0.0000 | - |
| 5 | ✗ FAIL | 1.9s | 1 | 3155 | 326 | 3155 | $0.0000 | - |
| 6 | ✓ PASS | 6.7s | 6 | 18635 | 1180 | 3738 | $0.0000 | Search, Write |
| 7 | ✗ FAIL | 1.0s | 1 | 3027 | 198 | 3027 | $0.0000 | - |
| 8 | ✗ FAIL | 10.3s | 2 | 8133 | 2475 | 4322 | $0.0000 | - |
| 9 | ✗ FAIL | 7.2s | 3 | 10070 | 1501 | 3988 | $0.0000 | Write |
| 10 | ✗ FAIL | 1.7s | 2 | 6015 | 357 | 3084 | $0.0000 | - |

**Failures:**

**Run 1**: file empty.txt does not exist

**Run 2**: file empty.txt does not exist

**Run 3**: file empty.txt does not exist

**Run 4**: file empty.txt does not exist

**Run 5**: file empty.txt does not exist

**Run 7**: file empty.txt does not exist

**Run 8**: file empty.txt does not exist

**Run 9**: file empty.txt does not exist

**Run 10**: file empty.txt does not exist

---

#### W6

**✗ Path with Spaces** | Handle paths with spaces | 0% (0/10) | 1.6 calls | 5109 tokens | 2.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 7.9s | 2 | 7349 | 1588 | 4426 | $0.0000 | Write |
| 2 | ✗ FAIL | 1.5s | 2 | 5979 | 232 | 3050 | $0.0000 | Search |
| 3 | ✗ FAIL | 1.4s | 1 | 3147 | 309 | 3147 | $0.0000 | - |
| 4 | ✗ FAIL | 2.7s | 1 | 3392 | 554 | 3392 | $0.0000 | - |
| 5 | ✗ FAIL | 5.7s | 4 | 13055 | 1222 | 3813 | $0.0000 | Write×2 |
| 6 | ✗ FAIL | 0.5s | 1 | 2928 | 90 | 2928 | $0.0000 | - |
| 7 | ✗ FAIL | 0.7s | 1 | 3008 | 170 | 3008 | $0.0000 | - |
| 8 | ✗ FAIL | 1.0s | 1 | 3080 | 242 | 3080 | $0.0000 | - |
| 9 | ✗ FAIL | 1.7s | 2 | 6105 | 318 | 3125 | $0.0000 | Write |
| 10 | ✗ FAIL | 0.9s | 1 | 3050 | 212 | 3050 | $0.0000 | - |

**Failures:**

**Run 1**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 2**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 3**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 4**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 5**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 6**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 7**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 8**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 9**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

**Run 10**: file my folder/my file.txt does not exist
failed to read file my folder/my file.txt: open /home/sk/kvit-coder/benchmarks/.kvit-coder-benchmark/workspace-4.2b/my folder/my file.txt: no such file or directory

---

## Failure Analysis

| Benchmark | Run | Errors | Last Tool Call |
|-----------|-----|--------|----------------|
| S1 | 1 | output does not contain 'func calculateTotal(it... | Search |
| S3 | 1 | output does not contain 'models.go' | Search |
| S4 | 1 | output does not contain 'folder3' | Search |
| R1 | 1 | output does not contain 'db_host=localhost' | Search |
| R2 | 1 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 1 | output does not contain 'SUBDIR_CONTENT' | Search |
| W1 | 1 | file hello.txt does not exist; failed to read f... | Search |
| W2 | 1 | file main.go does not exist; failed to read fil... |  |
| W3 | 1 | file existing.txt content does not match expect... | Search |
| W4 | 1 | failed to read file special.txt: open /home/sk/... |  |
| W5 | 1 | file empty.txt does not exist |  |
| W6 | 1 | file my folder/my file.txt does not exist; fail... | Write |
| E1 | 1 | cancelled | Search |
| E2 | 1 | file file.txt content does not match expected:
... |  |
| E3 | 1 | file data.txt content does not match expected:
... | Search |
| E4 | 1 | file boundary.txt content does not match expect... | Search |
| E5 | 1 | file code.ts content does not match expected:
-... | Search |
| E6 | 1 | file values.ts content does not match expected:... | Search |
| E7 | 1 | file func.ts content does not match expected:
-... |  |
| E9 | 1 | file numbered.txt content does not match expect... | Search |
| E10 | 1 | file special.txt content does not match expecte... |  |
| E11 | 1 | cancelled | Search |
| C1 | 1 | output does not contain 'db.example.com' | Search |
| C2 | 1 | file config.yaml content does not match expecte... | Search |
| C3 | 1 | file handler1.ts content does not match expecte... | Search |
| C5 | 1 | output does not contain 'wire.go' | Search |
| S1 | 2 | output does not contain 'func calculateTotal(it... | Search |
| S2 | 2 | output does not contain '7' | Search |
| S3 | 2 | output does not contain 'models.go' | Search |
| S4 | 2 | output does not contain 'folder3' | Search |
| R1 | 2 | output does not contain 'db_host=localhost' | Search |
| R2 | 2 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 2 | output does not contain 'SUBDIR_CONTENT' |  |
| W1 | 2 | file hello.txt does not exist; failed to read f... |  |
| W2 | 2 | file main.go does not exist; failed to read fil... |  |
| W3 | 2 | file existing.txt content does not match expect... |  |
| W4 | 2 | failed to read file special.txt: open /home/sk/... | Search |
| W5 | 2 | file empty.txt does not exist |  |
| W6 | 2 | file my folder/my file.txt does not exist; fail... | Search |
| E1 | 2 | cancelled | Search |
| E2 | 2 | file file.txt content does not match expected:
... |  |
| E3 | 2 | file data.txt content does not match expected:
... | Search |
| E4 | 2 | file boundary.txt content does not match expect... | Search |
| E5 | 2 | file code.ts content does not match expected:
-... | Search |
| E6 | 2 | file values.ts content does not match expected:... | Search |
| E7 | 2 | file func.ts content does not match expected:
-... | Search |
| E9 | 2 | file numbered.txt content does not match expect... |  |
| E10 | 2 | file special.txt content does not match expecte... | Search |
| E11 | 2 | file process.py content does not match expected... | Search |
| C1 | 2 | cancelled | Search |
| C2 | 2 | cancelled | Search |
| C3 | 2 | file handler1.ts content does not match expecte... | Search |
| C5 | 2 | output does not contain 'wire.go' | Search |
| S1 | 3 | output does not contain 'func calculateTotal(it... | Search |
| S3 | 3 | output does not contain 'models.go' | Search |
| S4 | 3 | output does not contain 'folder3' | Search |
| R1 | 3 | output does not contain 'db_host=localhost' | Write |
| R2 | 3 | cancelled | Search |
| R3 | 3 | output does not contain 'SUBDIR_CONTENT' | Search |
| W1 | 3 | file hello.txt does not exist; failed to read f... | Search |
| W2 | 3 | file main.go does not exist; failed to read fil... |  |
| W3 | 3 | file existing.txt content does not match expect... |  |
| W4 | 3 | failed to read file special.txt: open /home/sk/... |  |
| W5 | 3 | file empty.txt does not exist |  |
| W6 | 3 | file my folder/my file.txt does not exist; fail... |  |
| E1 | 3 | file numbered.txt content does not match expect... | Search |
| E2 | 3 | file file.txt content does not match expected:
... | Search |
| E3 | 3 | file data.txt content does not match expected:
... | Search |
| E4 | 3 | file boundary.txt content does not match expect... | Search |
| E5 | 3 | file code.ts content does not match expected:
-... | Search |
| E6 | 3 | file values.ts content does not match expected:... | Search |
| E7 | 3 | cancelled | Search |
| E9 | 3 | file numbered.txt content does not match expect... | Search |
| E10 | 3 | file special.txt content does not match expecte... | Write |
| E11 | 3 | file process.py content does not match expected... | Search |
| C1 | 3 | output does not contain 'db.example.com' | Search |
| C2 | 3 | file config.yaml content does not match expecte... | Search |
| C3 | 3 | file handler1.ts content does not match expecte... | Search |
| C5 | 3 | output does not contain 'wire.go' | Search |
| S1 | 4 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 4 | output does not contain '7' | Search |
| S3 | 4 | output does not contain 'models.go' | Search |
| S4 | 4 | output does not contain 'folder3' | Search |
| R1 | 4 | output does not contain 'db_host=localhost' | Search |
| R2 | 4 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 4 | cancelled | Search |
| W1 | 4 | file hello.txt does not exist; failed to read f... |  |
| W2 | 4 | file main.go does not exist; failed to read fil... |  |
| W3 | 4 | file existing.txt content does not match expect... |  |
| W4 | 4 | failed to read file special.txt: open /home/sk/... |  |
| W5 | 4 | file empty.txt does not exist |  |
| W6 | 4 | file my folder/my file.txt does not exist; fail... |  |
| E1 | 4 | cancelled | Edit |
| E2 | 4 | file file.txt content does not match expected:
... | Search |
| E3 | 4 | file data.txt content does not match expected:
... | Search |
| E4 | 4 | cancelled | Search |
| E5 | 4 | file code.ts content does not match expected:
-... | Search |
| E6 | 4 | cancelled | Search |
| E7 | 4 | file func.ts content does not match expected:
-... | Search |
| E9 | 4 | file numbered.txt content does not match expect... |  |
| E10 | 4 | file special.txt content does not match expecte... | Search |
| E11 | 4 | cancelled | Search |
| C1 | 4 | output does not contain 'db.example.com' | Search |
| C2 | 4 | file config.yaml content does not match expecte... | Search |
| C3 | 4 | cancelled | Search |
| C5 | 4 | output does not contain 'wire.go' | Search |
| S1 | 5 | output does not contain 'func calculateTotal(it... | Search |
| S2 | 5 | output does not contain '7' | Search |
| S3 | 5 | output does not contain 'models.go' |  |
| S4 | 5 | output does not contain 'folder3' | Search |
| R1 | 5 | output does not contain 'db_host=localhost' |  |
| R2 | 5 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 5 | output does not contain 'SUBDIR_CONTENT' | Search |
| W1 | 5 | file hello.txt does not exist; failed to read f... |  |
| W2 | 5 | file main.go does not exist; failed to read fil... |  |
| W3 | 5 | file existing.txt content does not match expect... | Search |
| W4 | 5 | failed to read file special.txt: open /home/sk/... | Write |
| W5 | 5 | file empty.txt does not exist |  |
| W6 | 5 | file my folder/my file.txt does not exist; fail... | Write |
| E1 | 5 | file numbered.txt content does not match expect... | Search |
| E2 | 5 | file file.txt content does not match expected:
... | Search |
| E3 | 5 | file data.txt content does not match expected:
... |  |
| E4 | 5 | file boundary.txt content does not match expect... | Search |
| E5 | 5 | file code.ts content does not match expected:
-... | Search |
| E6 | 5 | file values.ts content does not match expected:... | Search |
| E7 | 5 | cancelled | Search |
| E9 | 5 | file numbered.txt content does not match expect... | Search |
| E10 | 5 | file special.txt content does not match expecte... | Search |
| E11 | 5 | file process.py content does not match expected... | Write |
| C1 | 5 | output does not contain 'db.example.com' | Search |
| C2 | 5 | cancelled | Search |
| C3 | 5 | file handler1.ts content does not match expecte... | Search |
| C5 | 5 | output does not contain 'wire.go' | Search |
| S1 | 6 | output does not contain 'func calculateTotal(it... | Search |
| S2 | 6 | output does not contain '7' | Search |
| S3 | 6 | output does not contain 'models.go' | Search |
| S4 | 6 | cancelled | Search |
| R1 | 6 | output does not contain 'db_host=localhost' | Search |
| R2 | 6 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 6 | output does not contain 'SUBDIR_CONTENT' | Search |
| W2 | 6 | file main.go does not exist; failed to read fil... |  |
| W3 | 6 | file existing.txt content does not match expect... |  |
| W4 | 6 | failed to read file special.txt: open /home/sk/... |  |
| W6 | 6 | file my folder/my file.txt does not exist; fail... |  |
| E1 | 6 | file numbered.txt content does not match expect... | Search |
| E2 | 6 | file file.txt content does not match expected:
... |  |
| E3 | 6 | file data.txt content does not match expected:
... | Search |
| E4 | 6 | file boundary.txt content does not match expect... | Search |
| E5 | 6 | file code.ts content does not match expected:
-... | Search |
| E6 | 6 | file values.ts content does not match expected:... | Search |
| E7 | 6 | file func.ts content does not match expected:
-... | Search |
| E9 | 6 | file numbered.txt content does not match expect... |  |
| E10 | 6 | file special.txt content does not match expecte... |  |
| E11 | 6 | file process.py content does not match expected... | Search |
| C1 | 6 | output does not contain 'db.example.com' | Search |
| C2 | 6 | file config.yaml content does not match expecte... | Search |
| C3 | 6 | file handler1.ts content does not match expecte... | Search |
| C5 | 6 | output does not contain 'wire.go' | Search |
| S1 | 7 | output does not contain 'func calculateTotal(it... | Search |
| S2 | 7 | output does not contain '7' | Search |
| S3 | 7 | output does not contain 'models.go' | Search |
| S4 | 7 | output does not contain 'folder3' | Search |
| R1 | 7 | output does not contain 'db_host=localhost' | Search |
| R2 | 7 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 7 | output does not contain 'SUBDIR_CONTENT' | Search |
| W1 | 7 | file hello.txt does not exist; failed to read f... | Write.cancel |
| W2 | 7 | file main.go does not exist; failed to read fil... |  |
| W3 | 7 | file existing.txt content does not match expect... | Write |
| W4 | 7 | failed to read file special.txt: open /home/sk/... |  |
| W5 | 7 | file empty.txt does not exist |  |
| W6 | 7 | file my folder/my file.txt does not exist; fail... |  |
| E1 | 7 | file numbered.txt content does not match expect... | Search |
| E2 | 7 | file file.txt content does not match expected:
... | Search |
| E3 | 7 | cancelled | Search |
| E4 | 7 | file boundary.txt content does not match expect... | Search |
| E5 | 7 | file code.ts content does not match expected:
-... | Search |
| E6 | 7 | file values.ts content does not match expected:... | Search |
| E7 | 7 | file func.ts content does not match expected:
-... | Search |
| E8 | 7 | output contains 'replaced' but should not | Search |
| E9 | 7 | file numbered.txt content does not match expect... |  |
| E10 | 7 | file special.txt content does not match expecte... | Search |
| E11 | 7 | file process.py content does not match expected... |  |
| C1 | 7 | output does not contain 'db.example.com' | Search |
| C2 | 7 | file config.yaml content does not match expecte... | Read |
| C3 | 7 | file handler1.ts content does not match expecte... | Search |
| C5 | 7 | output does not contain 'wire.go' | Search |
| S1 | 8 | output does not contain 'func calculateTotal(it... | Search |
| S2 | 8 | output does not contain '7' | Search |
| S3 | 8 | cancelled | Search |
| S4 | 8 | output does not contain 'folder3' | Search |
| R1 | 8 | output does not contain 'db_host=localhost' | Search |
| R2 | 8 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 8 | output does not contain 'SUBDIR_CONTENT' | Search |
| W1 | 8 | file hello.txt does not exist; failed to read f... |  |
| W2 | 8 | file main.go does not exist; failed to read fil... |  |
| W3 | 8 | file existing.txt content does not match expect... | Search |
| W4 | 8 | file special.txt content does not match expecte... | Write |
| W5 | 8 | file empty.txt does not exist |  |
| W6 | 8 | file my folder/my file.txt does not exist; fail... |  |
| E1 | 8 | file numbered.txt content does not match expect... | Search |
| E2 | 8 | file file.txt content does not match expected:
... | Search |
| E3 | 8 | file data.txt content does not match expected:
... | Search |
| E4 | 8 | file boundary.txt content does not match expect... | Search |
| E5 | 8 | file code.ts content does not match expected:
-... | Search |
| E6 | 8 | file values.ts content does not match expected:... |  |
| E7 | 8 | file func.ts content does not match expected:
-... | Search |
| E9 | 8 | file numbered.txt content does not match expect... |  |
| E10 | 8 | file special.txt content does not match expecte... |  |
| E11 | 8 | file process.py content does not match expected... | Search |
| C1 | 8 | output does not contain 'db.example.com' | Search |
| C2 | 8 | cancelled | Search |
| C3 | 8 | file handler1.ts content does not match expecte... | Search |
| C5 | 8 | output does not contain 'wire.go' | Search |
| S1 | 9 | output does not contain 'func calculateTotal(it... | Search |
| S3 | 9 | output does not contain 'models.go' | Search |
| S4 | 9 | output does not contain 'folder3' | Search |
| R1 | 9 | output does not contain 'db_host=localhost' | Search |
| R2 | 9 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 9 | output does not contain 'SUBDIR_CONTENT' | Search |
| W1 | 9 | file hello.txt does not exist; failed to read f... |  |
| W2 | 9 | file main.go does not exist; failed to read fil... |  |
| W3 | 9 | file existing.txt content does not match expect... |  |
| W4 | 9 | failed to read file special.txt: open /home/sk/... |  |
| W5 | 9 | file empty.txt does not exist | Write |
| W6 | 9 | file my folder/my file.txt does not exist; fail... | Write |
| E1 | 9 | file numbered.txt content does not match expect... | Write |
| E2 | 9 | cancelled | Search |
| E3 | 9 | cancelled | Search |
| E4 | 9 | file boundary.txt content does not match expect... | Search |
| E5 | 9 | cancelled | Search |
| E6 | 9 | file values.ts content does not match expected:... | Search |
| E7 | 9 | cancelled | Search |
| E9 | 9 | file numbered.txt content does not match expect... |  |
| E10 | 9 | file special.txt content does not match expecte... | Write |
| E11 | 9 | file process.py content does not match expected... | Search |
| C1 | 9 | output does not contain 'db.example.com' | Search |
| C2 | 9 | file config.yaml content does not match expecte... |  |
| C3 | 9 | file handler1.ts content does not match expecte... | Search |
| C5 | 9 | output does not contain 'wire.go' | Search |
| S1 | 10 | output does not contain 'func calculateTotal(it... | Search |
| S3 | 10 | output does not contain 'models.go' |  |
| S4 | 10 | output does not contain 'folder3' | Search |
| R1 | 10 | output does not contain 'db_host=localhost' | Search |
| R2 | 10 | output does not contain 'SECTION_ALPHA'; output... | Search |
| R3 | 10 | output does not contain 'SUBDIR_CONTENT' | Search |
| W1 | 10 | file hello.txt does not exist; failed to read f... |  |
| W2 | 10 | file main.go does not exist; failed to read fil... | Search |
| W3 | 10 | file existing.txt content does not match expect... | Search |
| W4 | 10 | failed to read file special.txt: open /home/sk/... |  |
| W5 | 10 | file empty.txt does not exist |  |
| W6 | 10 | file my folder/my file.txt does not exist; fail... |  |
| E1 | 10 | file numbered.txt content does not match expect... |  |
| E2 | 10 | file file.txt content does not match expected:
... |  |
| E3 | 10 | file data.txt content does not match expected:
... | Search |
| E4 | 10 | file boundary.txt content does not match expect... | Search |
| E5 | 10 | file code.ts content does not match expected:
-... | Search |
| E6 | 10 | file values.ts content does not match expected:... | Search |
| E7 | 10 | file func.ts content does not match expected:
-... | Search |
| E9 | 10 | file numbered.txt content does not match expect... | Search |
| E10 | 10 | file special.txt content does not match expecte... | Write |
| E11 | 10 | cancelled | Search |
| C1 | 10 | output does not contain 'db.example.com' | Search |
| C2 | 10 | file config.yaml content does not match expecte... | Search |
| C3 | 10 | file handler1.ts content does not match expecte... |  |
| C5 | 10 | output does not contain 'wire.go' | Search |

## Appendix A: Configuration

### Version

```
kvit-coder 855d738 (commit 20260104, built 2026-01-04)
```

### config.yaml

```yaml
llm:
  base_url: "http://192.168.8.20:8080/v1"  # Your OpenAI-compatible endpoint
  model: "gpt-oss-4.2b-q80"
  context: 128000
  merge_thinking: false  # true = merge reasoning into content, false = discard
  verbose: 0             # 0 = off, >0 = show tool output up to N lines

workspace:
  root: "."
  path_safety_mode: "ask_once"  # "block", "warn", "ask_once", "ask_always"

agent:
  max_tool_iterations: 1000

backtrack:
  enabled: true             # Enable backtracking on semantic errors (LLM misuse)
  max_retries: 3            # Max retries at same history point before falling back to error-in-history
  inject_user_message: false # On limit reached: backtrack + inject user message (with error) instead of error-in-history

tools:
  # All tools are disabled by default - explicitly enable the ones you want

  read:
    enabled: true
    max_file_size_kb: 128
    max_read_size_kb: 24
    max_partial_lines: 150
    show_line_numbers: true     # set to false for content-based edit modes (searchreplace, patch)

  edit:
    enabled: true
    mode: "lines"               # "lines" (default), "searchreplace", or "patch"
    max_file_size_kb: 128
    preview_mode: true         # enables edit.confirm/edit.cancel
    read_before_edit_msgs: 0    # require read within N messages before edit (0 = disabled)
    pending_confirm_retries: 3  # max retries when LLM ignores confirm/cancel (0 = disabled, default 5)
    fuzzy_threshold: 0.0        # for searchreplace mode: 0 = exact only, 0.8 = enable fuzzy matching

  restore_file:
    enabled: true

  search:
    enabled: true
    max_snippet_results: 20   # Full snippets up to this many matches
    max_compact_results: 100  # file:line:match up to this many; above saves to temp file

  shell:
    enabled: true
    # Always blocked: sudo, su, apt, yum, brew, shutdown, reboot, chroot, mkfs, dd, sed -i, awk, cd
    allowed_commands: []        # allowlist (empty = allow all)
    disallowed_commands: []     # blocklist (checked after allowlist)

  plan:
    enabled: false              # group toggle for all plan.* tools
    injection_mode: "none"      # "none" or "every_step" - inject plan state after tool calls

  checkpoint:
    enabled: false              # group toggle for all checkpoint.* tools
    max_turns: 100
    temp_dir: ""
    max_file_size_kb: 1024
    excluded_patterns: []

  tasks:
    enabled: false              # Enable Tasks.* tools (disables Plan.* and Checkpoint.* tools)
    collapse: false             # Stage 2: Enable Tasks.Collapse
    plan: false                 # Stage 3: Enable plan-based Tasks tools
    # Runtime notice thresholds
    task_warn_turns: 5          # Warn after N turns in task
    task_critical_turns: 10     # Critical warning after N turns
    context_capacity_warn: 80   # Warn at N% context capacity
    max_nested_depth: 2         # Max task nesting depth warning
    notify_file_changes: true   # Notify about file changes in task

# Enhanced safety mode configuration (all features disabled by default)
safety:
  strict_mode: true            # Block unparseable commands (fail-closed)
  paranoid_mode: false          # Extra aggressive restrictions

  audit:
    enabled: false              # Enable audit logging of blocked commands
    log_dir: "~/.kvit-coder/safety-logs"
    redact_secrets: true        # Redact API keys, tokens from logs
    retention_days: 30

  git:
    block_push: false           # Block all git push (user should run manually)
    block_hard_reset: false     # Block git reset --hard
    block_checkout_discard: false  # Block git checkout -- <file>
    block_stash_drop: false     # Block git stash drop/clear
    block_clean_force: false    # Block git clean -f (allows -n/--dry-run)
    warn_branch_force_delete: false  # Warn on git branch -D

  rm:
    allow_in_temp: false        # Allow rm -rf in /tmp, /var/tmp
    allow_in_workspace_cwd: false  # Allow rm -rf in workspace subdirs
    block_workspace_root: false # Block rm -rf on workspace root

  interpreters:
    block_one_liners: false     # Block python -c, node -e, etc.
    allowed: []                 # Allowlist if blocking (e.g., ["python", "node"])

# Prompt template configuration
prompts:
  use_templates: false          # Enable template-based prompt generation (default: false uses hardcoded)
  templates_dir: ""             # Override embedded templates with filesystem directory (e.g., "./prompts")
  hot_reload: false             # Reload templates on each request (useful for development)

```

