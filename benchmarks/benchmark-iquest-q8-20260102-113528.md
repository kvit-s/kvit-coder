# LLM Tool Usage Benchmark Report

## Metadata

- **Version**: kvit-coder de939e8 (commit 20260101, built 2026-01-02)
- **Date**: 2026-01-02T13:58:41-06:00
- **Total Benchmarks**: 28
- **Total Runs**: 280

## Summary

| Class | Success Rate | Avg Time/Run |
|-------|--------------|--------------|
| C | 48% (19/40) | 246.0s |
| E | 17% (19/110) | 284.9s |
| R | 90% (27/30) | 55.5s |
| S | 35% (14/40) | 165.7s |
| W | 100% (60/60) | 85.0s |
| **Total** | **50% (139/280)** | **837.0s** |

## Detailed Statistics

### Per-Benchmark Summary

| Benchmark | Success | LLM Calls | Tokens | Generated | Context | Prompt Speed | Gen Speed | Cost | Duration |
|-----------|---------|-----------|--------|-----------|---------|--------------|-----------|------|----------|
| C1 | 70% | 4.1(±1.1) | 12993(±3821) | 146(±69) | 3478 | 537.9 t/s | 6.3 t/s | $0.0000 | 35.8s(±15.6) |
| C2 | 30% | 4.1(±1.4) | 12676(±4154) | 146(±64) | 3332 | 464.9 t/s | 5.4 t/s | $0.0000 | 34.8s(±14.1) |
| C3 | 30% | 7.3(±0.5) | 29538(±2395) | 446(±16) | 4989 | 2815.8 t/s | 15.1 t/s | $0.0000 | 119.8s(±0.6) |
| C5 | 60% | 3.2(±2.0) | 11235(±6454) | 196(±122) | 4403 | 3574.4 t/s | 15.9 t/s | $0.0000 | 55.5s(±35.5) |
| E1 | 100% | 2.0 | 6145(±8) | 75(±8) | 3185 | 275.4 t/s | 4.9 t/s | $0.0000 | 18.1s(±1.7) |
| E2 | 0% | 2.5(±0.5) | 8149(±1878) | 143(±45) | 3517 | 2117.1 t/s | 12.7 t/s | $0.0000 | 35.6s(±11.5) |
| E3 | 0% | 3.1(±0.5) | 10101(±1938) | 118(±22) | 3550 | 661.8 t/s | 6.2 t/s | $0.0000 | 30.1s(±5.9) |
| E4 | 0% | 2.0 | 6101(±12) | 87(±12) | 3145 | 272.7 t/s | 5.0 t/s | $0.0000 | 20.6s(±2.6) |
| E5 | 0% | 2.5(±1.2) | 8224(±4957) | 160(±120) | 3424 | 2551.4 t/s | 10.6 t/s | $0.0000 | 38.3s(±29.1) |
| E6 | 10% | 2.3(±0.6) | 7168(±2345) | 101(±46) | 3230 | 685.6 t/s | 6.3 t/s | $0.0000 | 24.3s(±11.6) |
| E7 | 0% | 2.0 | 6093(±8) | 82(±8) | 3132 | 608.0 t/s | 5.7 t/s | $0.0000 | 19.2s(±1.7) |
| E8 | 0% | 2.1(±0.3) | 6396(±983) | 75(±16) | 3140 | 285.2 t/s | 5.0 t/s | $0.0000 | 17.9s(±3.9) |
| E9 | 80% | 2.8(±0.4) | 8879(±1379) | 104(±16) | 3378 | 322.4 t/s | 5.1 t/s | $0.0000 | 25.6s(±4.1) |
| E10 | 0% | 2.0 | 6115(±10) | 86(±8) | 3154 | 274.0 t/s | 4.8 t/s | $0.0000 | 20.4s(±1.8) |
| E11 | 0% | 2.6(±1.3) | 8388(±4860) | 142(±108) | 3336 | 1096.0 t/s | 7.1 t/s | $0.0000 | 34.9s(±29.0) |
| R1 | 90% | 2.0 | 6044(±4) | 63(±4) | 3123 | 294.6 t/s | 5.0 t/s | $0.0000 | 15.3s(±0.8) |
| R2 | 80% | 2.1(±0.3) | 7113(±1156) | 103(±22) | 3810 | 17396.2 t/s | 23.0 t/s | $0.0000 | 28.8s(±5.0) |
| R3 | 100% | 2.0 | 6010(±2) | 46(±2) | 3083 | 98.6 t/s | 4.7 t/s | $0.0000 | 11.3s(±0.6) |
| S1 | 10% | 3.3(±0.8) | 10770(±2889) | 156(±59) | 3570 | 1227.6 t/s | 9.2 t/s | $0.0000 | 34.7s(±13.4) |
| S2 | 10% | 2.0 | 5984(±89) | 76(±44) | 3019 | 865.8 t/s | 10.9 t/s | $0.0000 | 15.5s(±8.9) |
| S3 | 90% | 2.0 | 6006(±11) | 65(±6) | 3026 | 669.4 t/s | 6.6 t/s | $0.0000 | 13.4s(±1.2) |
| S4 | 30% | 4.1(±0.8) | 17496(±5347) | 308(±99) | 5481 | 19910.7 t/s | 34.5 t/s | $0.0000 | 102.0s(±26.7) |
| W1 | 100% | 2.0 | 5954(±5) | 53(±5) | 3019 | 84.3 t/s | 4.6 t/s | $0.0000 | 12.3s(±1.1) |
| W2 | 100% | 2.0 | 6014(±7) | 82(±7) | 3052 | 885.3 t/s | 5.7 t/s | $0.0000 | 18.6s(±1.6) |
| W3 | 100% | 3.0 | 9141(±7) | 68(±7) | 3147 | 91.1 t/s | 4.7 t/s | $0.0000 | 16.7s(±1.6) |
| W4 | 100% | 2.0 | 5982(±1) | 52(±1) | 3029 | 87.9 t/s | 4.7 t/s | $0.0000 | 12.2s(±0.2) |
| W5 | 100% | 2.0 | 5944(±42) | 57(±42) | 3018 | 381.2 t/s | 6.5 t/s | $0.0000 | 13.2s(±9.2) |
| W6 | 100% | 2.0 | 5968(±1) | 52(±1) | 3026 | 88.2 t/s | 4.7 t/s | $0.0000 | 12.1s(±0.3) |

### Per-Benchmark Details

#### C1

**✗ Search Then Read** | Find and read a file | 70% (7/10) | 4.1 calls | 12993 tokens | 35.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 24.5s | 4 | 12426 | 97 | 3377 | $0.0000 | Read×2, Search |
| 2 | ✓ PASS | 35.4s | 3 | 9569 | 146 | 3476 | $0.0000 | Read, Search |
| 3 | ✗ FAIL | 22.1s | 3 | 9175 | 91 | 3241 | $0.0000 | Read, Shell |
| 4 | ✓ PASS | 25.1s | 4 | 12424 | 95 | 3375 | $0.0000 | Read×2, Search |
| 5 | ✓ PASS | 77.7s | 7 | 22761 | 333 | 3766 | $0.0000 | Read×2, Search×4 |
| 6 | ✗ FAIL | 45.5s | 5 | 16364 | 181 | 3732 | $0.0000 | Read×2, Search×2 |
| 7 | ✓ PASS | 32.2s | 4 | 12500 | 130 | 3428 | $0.0000 | Read×2, Search |
| 8 | ✗ FAIL | 32.7s | 4 | 12706 | 130 | 3516 | $0.0000 | Read, Search×2 |
| 9 | ✓ PASS | 24.9s | 4 | 12428 | 99 | 3379 | $0.0000 | Read×2, Search |
| 10 | ✓ PASS | 38.0s | 3 | 9581 | 158 | 3488 | $0.0000 | Read, Search |

**Failures:**

**Run 3**: output does not contain 'db.example.com'

**Run 6**: output does not contain 'db.example.com'

**Run 8**: output does not contain 'db.example.com'

---

#### C2

**✗ Read-Modify-Write** | Complete edit workflow | 30% (3/10) | 4.1 calls | 12676 tokens | 34.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 23.3s | 3 | 9370 | 94 | 3309 | $0.0000 | Edit, Read |
| 2 | ✗ FAIL | 35.7s | 4 | 12388 | 149 | 3368 | $0.0000 | Edit, Read |
| 3 | ✗ FAIL | 43.6s | 5 | 15277 | 187 | 3313 | $0.0000 | Edit, Read |
| 4 | ✓ PASS | 25.2s | 3 | 9391 | 103 | 3323 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 23.9s | 3 | 9373 | 97 | 3312 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 69.5s | 7 | 21619 | 300 | 3509 | $0.0000 | Edit, Read, Shell |
| 7 | ✓ PASS | 23.1s | 3 | 9381 | 93 | 3323 | $0.0000 | Edit, Read |
| 8 | ✗ FAIL | 23.9s | 3 | 9372 | 96 | 3311 | $0.0000 | Edit, Read |
| 9 | ✓ PASS | 45.6s | 6 | 18263 | 198 | 3244 | $0.0000 | Shell, Write, Write.confirm |
| 10 | ✗ FAIL | 34.4s | 4 | 12329 | 145 | 3313 | $0.0000 | Edit, Read |

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
+    port: 8080
 

```


**Run 2**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-server:
-  host: localhost
-  port: 8080
+    server:
+      host: localhost
+      port: 8080
 

```


**Run 3**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+    port: 8080
 

```


**Run 5**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+    port: 8080
 

```


**Run 6**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-server:
-  host: localhost
-  port: 8080
+   1│server:
+   2│  host: localhost
+   3│  port: 8080
 

```


**Run 8**: 
```diff
file config.yaml content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
 server:
   host: localhost
-  port: 8080
+    port: 8080
 

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
+    port: 8080
 

```


---

#### C3

**✗ Search-Read-Edit** | Find, understand, and modify | 30% (3/10) | 7.3 calls | 29538 tokens | 119.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 120.0s | 7 | 28006 | 441 | 4886 | $0.0000 | Edit×3, Read×3, Search |
| 2 | ✗ FAIL | 120.0s | 7 | 28001 | 438 | 4891 | $0.0000 | Edit×3, Read×3, Search |
| 3 | ✗ FAIL | 120.0s | 7 | 28000 | 437 | 4891 | $0.0000 | Edit×3, Read×3, Search |
| 4 | ✗ FAIL | 120.0s | 7 | 28005 | 442 | 4891 | $0.0000 | Edit×3, Read×3, Search |
| 5 | ✓ PASS | 120.0s | 7 | 27788 | 432 | 4885 | $0.0000 | Edit×3, Read×3, Search |
| 6 | ✗ FAIL | 120.0s | 7 | 27998 | 433 | 4886 | $0.0000 | Edit×3, Read×3, Search |
| 7 | ✗ FAIL | 120.0s | 8 | 33242 | 454 | 5243 | $0.0000 | Edit×3, Read×3, Search×2 |
| 8 | ✓ PASS | 118.0s | 8 | 33087 | 481 | 5185 | $0.0000 | Edit×3, Read×3, Search |
| 9 | ✗ FAIL | 120.0s | 8 | 33255 | 467 | 5243 | $0.0000 | Edit×3, Read×3, Search×2 |
| 10 | ✗ FAIL | 120.0s | 7 | 27993 | 430 | 4891 | $0.0000 | Edit×3, Read×3, Search |

**Failures:**

**Run 2**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -5,3 +5,6 @@
 function deprecatedFunc() {}
 function newFunc() {}
 
+function deprecatedFunc() {}
+function newFunc() {}
+

```


**Run 3**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -5,3 +5,6 @@
 function deprecatedFunc() {}
 function newFunc() {}
 
+function deprecatedFunc() {}
+function newFunc() {}
+

```


**Run 4**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -4,4 +4,8 @@
 
 function deprecatedFunc() {}
 function newFunc() {}
+}
 
+function deprecatedFunc() {}
+function newFunc() {}
+

```


**Run 6**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -4,4 +4,8 @@
 
 function deprecatedFunc() {}
 function newFunc() {}
+}
 
+function deprecatedFunc() {}
+function newFunc() {}
+

```


**Run 7**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -5,3 +5,6 @@
 function deprecatedFunc() {}
 function newFunc() {}
 
+function deprecatedFunc() {}
+function newFunc() {}
+

```


**Run 9**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -5,3 +5,6 @@
 function deprecatedFunc() {}
 function newFunc() {}
 
+function deprecatedFunc() {}
+function newFunc() {}
+

```


**Run 10**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -5,3 +5,6 @@
 function deprecatedFunc() {}
 function newFunc() {}
 
+function deprecatedFunc() {}
+function newFunc() {}
+

```


---

#### C5

**✗ Needle in Haystack Search** | Find specific initialization among many usages | 60% (6/10) | 3.2 calls | 11235 tokens | 55.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 52.5s | 2 | 8118 | 161 | 5189 | $0.0000 | Search |
| 2 | ✗ FAIL | 31.4s | 2 | 6077 | 141 | 3101 | $0.0000 | Search |
| 3 | ✗ FAIL | 16.0s | 2 | 5967 | 71 | 3031 | $0.0000 | Search |
| 4 | ✓ PASS | 48.1s | 8 | 26536 | 180 | 3916 | $0.0000 | Read×4, Search×3 |
| 5 | ✓ PASS | 71.9s | 2 | 8209 | 252 | 5280 | $0.0000 | Search |
| 6 | ✓ PASS | 120.0s | 5 | 18083 | 424 | 5606 | $0.0000 | Read, Search×4 |
| 7 | ✗ FAIL | 34.8s | 2 | 7049 | 117 | 4123 | $0.0000 | Search |
| 8 | ✗ FAIL | 120.0s | 5 | 16195 | 415 | 3530 | $0.0000 | Search×5 |
| 9 | ✓ PASS | 37.7s | 2 | 8051 | 94 | 5122 | $0.0000 | Search |
| 10 | ✓ PASS | 22.9s | 2 | 8062 | 105 | 5133 | $0.0000 | Search |

**Failures:**

**Run 2**: output does not contain 'wire.go'

**Run 3**: output does not contain 'wire.go'

**Run 7**: output does not contain 'wire.go'

**Run 8**: output does not contain 'wire.go'

---

#### E1

**✓ Single Line Replace** | Replace a single line | 100% (10/10) | 2.0 calls | 6145 tokens | 18.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 17.4s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 2 | ✓ PASS | 17.3s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 3 | ✓ PASS | 17.8s | 2 | 6145 | 73 | 3183 | $0.0000 | Edit |
| 4 | ✓ PASS | 17.1s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 5 | ✓ PASS | 18.6s | 2 | 6147 | 77 | 3187 | $0.0000 | Edit |
| 6 | ✓ PASS | 23.0s | 2 | 6167 | 97 | 3207 | $0.0000 | Edit |
| 7 | ✓ PASS | 17.7s | 2 | 6145 | 73 | 3183 | $0.0000 | Edit |
| 8 | ✓ PASS | 17.3s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 9 | ✓ PASS | 17.3s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 10 | ✓ PASS | 17.3s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |

---

#### E2

**✗ Multi-line Insert** | Insert multiple lines at position | 0% (0/10) | 2.5 calls | 8149 tokens | 35.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 29.4s | 2 | 6311 | 122 | 3322 | $0.0000 | Edit |
| 2 | ✗ FAIL | 24.7s | 2 | 6220 | 102 | 3234 | $0.0000 | Edit |
| 3 | ✗ FAIL | 37.1s | 3 | 9879 | 146 | 3631 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 50.0s | 3 | 9967 | 204 | 3713 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 23.6s | 2 | 6256 | 96 | 3276 | $0.0000 | Edit |
| 6 | ✗ FAIL | 31.3s | 3 | 9842 | 120 | 3598 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 38.3s | 2 | 6382 | 162 | 3371 | $0.0000 | Edit |
| 8 | ✗ FAIL | 61.1s | 3 | 10486 | 238 | 4132 | $0.0000 | Edit, Read |
| 9 | ✗ FAIL | 36.6s | 3 | 9921 | 144 | 3655 | $0.0000 | Edit, Read |
| 10 | ✗ FAIL | 23.6s | 2 | 6222 | 98 | 3240 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,13 +8,13 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M
 FUNCTION N
+        inserted_line_1
+        inserted_line_2
+        inserted_line_3
 FUNCTION O
 FUNCTION P
 FUNCTION Q

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
@@ -21,4 +18,6 @@
 FUNCTION R
 FUNCTION S
 FUNCTION T
-
+        inserted
+        inserted
+        inserted

```


**Run 3**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,10 +7,9 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
-INSERTED
-INSERTED
-INSERTED
+        INSERTED
+        INSERTED
+        INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 4**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,10 +7,10 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
-INSERTED
-INSERTED
-INSERTED
+        INSERTED
+        INSERTED
+        INSERTED
+      
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 5**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,10 +8,10 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
+        INSERTED
+        INSERTED
+        INSERTED
 FUNCTION L
 FUNCTION M
 FUNCTION N

```


**Run 6**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,10 +7,9 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
-INSERTED
-INSERTED
-INSERTED
+       INSERTED
+       INSERTED
+       INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 7**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,8 @@
 FUNCTION A
 FUNCTION B
-FUNCTION C
+                INSERTED
+                INSERTED
+                INSERTED
 FUNCTION D
 FUNCTION E
 FUNCTION F
@@ -8,9 +10,6 @@
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
@@ -7,18 +7,18 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
-INSERTED
-INSERTED
-INSERTED
-FUNCTION K
-FUNCTION L
-FUNCTION M
-FUNCTION N
-FUNCTION O
-FUNCTION P
-FUNCTION Q
-FUNCTION R
-FUNCTION S
-FUNCTION T
+   10│FUNCTION J
+   11│INSERTED
+   12│INSERTED
+   13│INSERTED
+   14│FUNCTION K
+   15│FUNCTION L
+   16│FUNCTION M
+   17│FUNCTION N
+   18│FUNCTION O
+   19│FUNCTION P
+   20│FUNCTION Q
+   21│FUNCTION R
+   22│FUNCTION S
+   23│FUNCTION T
 

```


**Run 9**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,10 +7,10 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
-INSERTED
-INSERTED
-INSERTED
+        INSERTED
+        INSERTED
+        INSERTED
+        
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


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
@@ -22,3 +19,7 @@
 FUNCTION S
 FUNCTION T
 
+        INSERTED
+        INSERTED
+        INSERTED
+

```


---

#### E3

**✗ Delete Lines** | Delete a range of lines | 0% (0/10) | 3.1 calls | 10101 tokens | 30.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 28.1s | 3 | 9739 | 109 | 3527 | $0.0000 | Edit, Read |
| 2 | ✗ FAIL | 28.4s | 3 | 9741 | 111 | 3529 | $0.0000 | Edit, Read |
| 3 | ✗ FAIL | 26.9s | 3 | 9734 | 104 | 3522 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 28.1s | 3 | 9739 | 109 | 3527 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 27.0s | 3 | 9734 | 104 | 3522 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 26.6s | 3 | 9732 | 102 | 3520 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 29.4s | 3 | 9753 | 115 | 3531 | $0.0000 | Edit, Read |
| 8 | ✗ FAIL | 35.7s | 4 | 13224 | 139 | 3706 | $0.0000 | Edit, Read×2 |
| 9 | ✗ FAIL | 45.8s | 4 | 13460 | 177 | 3921 | $0.0000 | Edit, Read×2 |
| 10 | ✗ FAIL | 25.4s | 2 | 6151 | 109 | 3197 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


**Run 2**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


**Run 3**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


**Run 4**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


**Run 5**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


**Run 6**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


**Run 7**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


**Run 8**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


**Run 9**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,20 @@
 function H
 function I
 function J
+
+
+
+
+
+
+
+
+
+
+
+
+
+
 function O
 function P
 function Q

```


**Run 10**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -1,13 +1,15 @@
 function A
 function B
-function C
-function D
-function E
+
 function F
 function G
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

**✗ Boundary Edit** | Test boundary conditions | 0% (0/10) | 2.0 calls | 6101 tokens | 20.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 19.9s | 2 | 6097 | 83 | 3141 | $0.0000 | Edit |
| 2 | ✗ FAIL | 26.0s | 2 | 6126 | 112 | 3170 | $0.0000 | Edit |
| 3 | ✗ FAIL | 23.0s | 2 | 6112 | 98 | 3156 | $0.0000 | Edit |
| 4 | ✗ FAIL | 18.4s | 2 | 6091 | 77 | 3135 | $0.0000 | Edit |
| 5 | ✗ FAIL | 18.0s | 2 | 6089 | 75 | 3133 | $0.0000 | Edit |
| 6 | ✗ FAIL | 18.2s | 2 | 6090 | 76 | 3134 | $0.0000 | Edit |
| 7 | ✗ FAIL | 23.0s | 2 | 6112 | 98 | 3156 | $0.0000 | Edit |
| 8 | ✗ FAIL | 21.0s | 2 | 6103 | 89 | 3147 | $0.0000 | Edit |
| 9 | ✗ FAIL | 18.2s | 2 | 6090 | 76 | 3134 | $0.0000 | Edit |
| 10 | ✗ FAIL | 20.0s | 2 | 6098 | 84 | 3142 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
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
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
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
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 4**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


**Run 5**: 
```diff
file boundary.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
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
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
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
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
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
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
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
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
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
@@ -1,4 +1,4 @@
-function A
+cleanup done
 function B
 function C
 function D
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


---

#### E5

**✗ Replace All Occurrences** | Replace multiple occurrences | 0% (0/10) | 2.5 calls | 8224 tokens | 38.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 115.9s | 6 | 22703 | 476 | 4612 | $0.0000 | Edit×3, Read×2 |
| 2 | ✗ FAIL | 21.3s | 2 | 6146 | 90 | 3191 | $0.0000 | Edit |
| 3 | ✗ FAIL | 21.9s | 2 | 6204 | 91 | 3237 | $0.0000 | Edit |
| 4 | ✗ FAIL | 23.8s | 2 | 6157 | 101 | 3202 | $0.0000 | Edit |
| 5 | ✗ FAIL | 24.7s | 2 | 6215 | 103 | 3247 | $0.0000 | Edit |
| 6 | ✗ FAIL | 68.4s | 3 | 9977 | 289 | 3772 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 23.1s | 2 | 6203 | 96 | 3238 | $0.0000 | Edit |
| 8 | ✗ FAIL | 32.2s | 2 | 6196 | 140 | 3241 | $0.0000 | Edit |
| 9 | ✗ FAIL | 27.2s | 2 | 6222 | 115 | 3257 | $0.0000 | Edit |
| 10 | ✗ FAIL | 24.1s | 2 | 6213 | 100 | 3246 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -6,7 +6,7 @@
     const value = newFunc();
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
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+newFunc
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
@@ -1,12 +1,14 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+    return 'new';
+}
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
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+newFunc
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
@@ -1,12 +1,5 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
-        return;
-    }
-    const value = newFunc();
+function newFunc() {
+    // function implementation
+}
 }
 
-function newFunc(): string | null {
-    return null;
-}
-

```


**Run 6**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -6,7 +6,7 @@
     const value = newFunc();
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
@@ -1,12 +1,14 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+  // function body
+}
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
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+newFunc
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


**Run 9**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,14 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+  // function body
+}
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


**Run 10**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,14 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+    // function implementation
+}
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

**✗ Context-Specific Replace** | Replace with context for uniqueness | 10% (1/10) | 2.3 calls | 7168 tokens | 24.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 20.6s | 2 | 6084 | 88 | 3130 | $0.0000 | Edit |
| 2 | ✗ FAIL | 19.2s | 2 | 6078 | 82 | 3124 | $0.0000 | Edit |
| 3 | ✗ FAIL | 16.1s | 2 | 6064 | 68 | 3110 | $0.0000 | Edit |
| 4 | ✗ FAIL | 17.1s | 2 | 6034 | 68 | 3080 | $0.0000 | Edit |
| 5 | ✗ FAIL | 18.1s | 2 | 6074 | 78 | 3120 | $0.0000 | Edit |
| 6 | ✓ PASS | 38.1s | 3 | 9680 | 155 | 3533 | $0.0000 | Edit, Search |
| 7 | ✗ FAIL | 18.6s | 2 | 6076 | 80 | 3122 | $0.0000 | Edit |
| 8 | ✗ FAIL | 20.3s | 2 | 6083 | 87 | 3129 | $0.0000 | Edit |
| 9 | ✗ FAIL | 20.0s | 2 | 6082 | 86 | 3128 | $0.0000 | Edit |
| 10 | ✗ FAIL | 54.3s | 4 | 13420 | 220 | 3823 | $0.0000 | Edit, Read, Search |

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
-const data = { value: 3 };
+  const config = { value: 99 };
+  const data = { value: 3 };
 

```


---

#### E7

**✗ Multi-line Block Replace** | Replace multi-line block | 0% (0/10) | 2.0 calls | 6093 tokens | 19.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 22.7s | 2 | 6108 | 98 | 3148 | $0.0000 | Edit |
| 2 | ✗ FAIL | 19.1s | 2 | 6092 | 82 | 3132 | $0.0000 | Edit |
| 3 | ✗ FAIL | 20.9s | 2 | 6103 | 89 | 3141 | $0.0000 | Edit |
| 4 | ✗ FAIL | 18.1s | 2 | 6088 | 78 | 3128 | $0.0000 | Edit |
| 5 | ✗ FAIL | 17.0s | 2 | 6081 | 71 | 3121 | $0.0000 | Edit |
| 6 | ✗ FAIL | 17.2s | 2 | 6084 | 74 | 3124 | $0.0000 | Edit |
| 7 | ✗ FAIL | 18.3s | 2 | 6090 | 76 | 3128 | $0.0000 | Edit |
| 8 | ✗ FAIL | 19.4s | 2 | 6093 | 83 | 3133 | $0.0000 | Edit |
| 9 | ✗ FAIL | 18.2s | 2 | 6088 | 78 | 3128 | $0.0000 | Edit |
| 10 | ✗ FAIL | 21.0s | 2 | 6103 | 89 | 3141 | $0.0000 | Edit |

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

**✗ No Match Handling** | Handle search text not found | 0% (0/10) | 2.1 calls | 6396 tokens | 17.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 16.9s | 2 | 6069 | 71 | 3125 | $0.0000 | Edit |
| 2 | ✗ FAIL | 21.5s | 2 | 6090 | 92 | 3146 | $0.0000 | Edit |
| 3 | ✗ FAIL | 16.2s | 2 | 6066 | 68 | 3122 | $0.0000 | Edit |
| 4 | ✗ FAIL | 14.7s | 2 | 6059 | 61 | 3115 | $0.0000 | Edit |
| 5 | ✗ FAIL | 14.6s | 2 | 6059 | 61 | 3115 | $0.0000 | Edit |
| 6 | ✗ FAIL | 28.0s | 3 | 9345 | 117 | 3278 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 16.2s | 2 | 6066 | 68 | 3122 | $0.0000 | Edit |
| 8 | ✗ FAIL | 15.5s | 2 | 6063 | 65 | 3119 | $0.0000 | Edit |
| 9 | ✗ FAIL | 18.1s | 2 | 6075 | 77 | 3131 | $0.0000 | Edit |
| 10 | ✗ FAIL | 16.8s | 2 | 6069 | 71 | 3125 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```

output contains 'successfully' but should not

**Run 2**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```


**Run 3**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```

output contains 'successfully' but should not

**Run 4**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```


**Run 5**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```

output contains 'successfully' but should not

**Run 6**: output contains 'replaced' but should not
output contains 'successfully' but should not

**Run 7**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```

output contains 'successfully' but should not

**Run 8**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```

output contains 'successfully' but should not

**Run 9**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```

output contains 'replaced' but should not
output contains 'successfully' but should not

**Run 10**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```

output contains 'successfully' but should not

---

#### E9

**✗ Empty Content** | Handle empty replacement | 80% (8/10) | 2.8 calls | 8879 tokens | 25.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 16.1s | 2 | 6103 | 66 | 3151 | $0.0000 | Edit |
| 2 | ✓ PASS | 31.4s | 3 | 9589 | 129 | 3451 | $0.0000 | Edit, Read |
| 3 | ✓ PASS | 26.2s | 3 | 9563 | 105 | 3427 | $0.0000 | Edit, Read |
| 4 | ✓ PASS | 25.3s | 3 | 9559 | 101 | 3422 | $0.0000 | Edit, Read |
| 5 | ✓ PASS | 25.9s | 3 | 9561 | 103 | 3424 | $0.0000 | Edit, Read |
| 6 | ✓ PASS | 30.1s | 3 | 9581 | 123 | 3445 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 21.4s | 2 | 6141 | 91 | 3179 | $0.0000 | Edit |
| 8 | ✓ PASS | 26.8s | 3 | 9565 | 107 | 3428 | $0.0000 | Edit, Read |
| 9 | ✓ PASS | 26.8s | 3 | 9566 | 108 | 3429 | $0.0000 | Edit, Read |
| 10 | ✓ PASS | 25.8s | 3 | 9562 | 104 | 3426 | $0.0000 | Edit, Read |

**Failures:**

**Run 1**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -1,8 +1,9 @@
 LINE A
 LINE B
+
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
@@ -1,8 +1,8 @@
 LINE A
 LINE B
-LINE C
+
 LINE D
-
+LINE E
 LINE F
 LINE G
 LINE H

```


---

#### E10

**✗ Special Characters** | Handle special characters in replacement | 0% (0/10) | 2.0 calls | 6115 tokens | 20.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 19.5s | 2 | 6109 | 82 | 3149 | $0.0000 | Edit |
| 2 | ✗ FAIL | 19.5s | 2 | 6109 | 82 | 3149 | $0.0000 | Edit |
| 3 | ✗ FAIL | 21.8s | 2 | 6120 | 93 | 3160 | $0.0000 | Edit |
| 4 | ✗ FAIL | 19.8s | 2 | 6111 | 84 | 3151 | $0.0000 | Edit |
| 5 | ✗ FAIL | 22.9s | 2 | 6124 | 97 | 3164 | $0.0000 | Edit |
| 6 | ✗ FAIL | 16.8s | 2 | 6097 | 70 | 3137 | $0.0000 | Edit |
| 7 | ✗ FAIL | 22.0s | 2 | 6135 | 94 | 3167 | $0.0000 | Edit |
| 8 | ✗ FAIL | 22.7s | 2 | 6124 | 97 | 3164 | $0.0000 | Edit |
| 9 | ✗ FAIL | 19.6s | 2 | 6110 | 83 | 3150 | $0.0000 | Edit |
| 10 | ✗ FAIL | 19.4s | 2 | 6109 | 82 | 3149 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,6 +1,7 @@
+value = $100 + 50%
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
@@ -1,6 +1,7 @@
+value = $100 + 50%
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
@@ -1,6 +1,7 @@
+value = $100 + 50%
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
@@ -1,6 +1,7 @@
+value = $100 + 50%
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
@@ -1,6 +1,7 @@
+value = $100 + 50%
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
@@ -1,6 +1,7 @@
+value = $100 + 50%
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
-line A
+value = $100 + 50%
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
@@ -1,6 +1,7 @@
+value = $100 + 50%
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
@@ -1,6 +1,7 @@
+value = $100 + 50%
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
@@ -1,6 +1,7 @@
+value = $100 + 50%
 line A
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


---

#### E11

**✗ Indentation Preservation** | Maintain correct indentation (critical for Python) | 0% (0/10) | 2.6 calls | 8388 tokens | 34.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 23.7s | 2 | 6217 | 98 | 3241 | $0.0000 | Edit |
| 2 | ✗ FAIL | 20.4s | 2 | 6212 | 84 | 3232 | $0.0000 | Edit |
| 3 | ✗ FAIL | 120.0s | 6 | 21968 | 454 | 4185 | $0.0000 | Edit, Read×3, Shell×2 |
| 4 | ✗ FAIL | 26.7s | 2 | 6250 | 110 | 3267 | $0.0000 | Edit |
| 5 | ✗ FAIL | 20.8s | 2 | 6207 | 86 | 3231 | $0.0000 | Edit |
| 6 | ✗ FAIL | 23.7s | 2 | 6214 | 99 | 3238 | $0.0000 | Edit |
| 7 | ✗ FAIL | 26.7s | 2 | 6230 | 113 | 3254 | $0.0000 | Edit |
| 8 | ✗ FAIL | 20.0s | 2 | 6206 | 83 | 3238 | $0.0000 | Edit |
| 9 | ✗ FAIL | 42.5s | 4 | 12150 | 185 | 3227 | $0.0000 | Edit |
| 10 | ✗ FAIL | 24.7s | 2 | 6224 | 105 | 3248 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,7 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+validate_input()
+transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

**Run 2**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,7 @@
 def outer():
     if True:
-        process_data()
-        validate_input()
-        transform_data()
+validate_input()
+    transform_data()
+        # placeholder
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: expected an indented block after 'if' statement on line 2 (process.py, line 3)

**Run 3**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,7 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
-        finalize()
+        # placeholder
+validate_input()
+    transform_data()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

**Run 4**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,8 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
+    validate_input()
+    transform_data()
 

```


**Run 5**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,7 @@
 def outer():
-    if True:
+validate_input()
+transform_data()
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: expected an indented block after function definition on line 1 (process.py, line 2)

**Run 6**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,7 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
-        finalize()
+        # placeholder
+validate_input()
+transform_data()
 

```


**Run 7**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,7 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
-
+validate_input()
+transform_data()

```


**Run 8**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,8 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
+validate_input()
+transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 7)

**Run 9**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,7 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+        # placeholder
         finalize()
-
+validate_input()
+transform_data()

```


**Run 10**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,7 @@
 def outer():
     if True:
         process_data()
-        validate_input()
-        transform_data()
+validate_input()
+transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

---

#### R1

**✗ Simple File Read** | Read an entire small file | 90% (9/10) | 2.0 calls | 6044 tokens | 15.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 15.1s | 2 | 6041 | 60 | 3120 | $0.0000 | Read |
| 2 | ✓ PASS | 15.8s | 2 | 6047 | 66 | 3126 | $0.0000 | Read |
| 3 | ✓ PASS | 14.5s | 2 | 6041 | 60 | 3120 | $0.0000 | Read |
| 4 | ✓ PASS | 14.7s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 5 | ✓ PASS | 15.7s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 6 | ✓ PASS | 14.7s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 7 | ✓ PASS | 14.9s | 2 | 6043 | 62 | 3122 | $0.0000 | Read |
| 8 | ✓ PASS | 14.7s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 9 | ✓ PASS | 16.3s | 2 | 6049 | 68 | 3128 | $0.0000 | Read |
| 10 | ✗ FAIL | 17.0s | 2 | 6052 | 71 | 3131 | $0.0000 | Read |

**Failures:**

**Run 10**: output does not contain 'db_host=localhost'

---

#### R2

**✗ Truncation Recovery** | Handle truncated output by chunked reading | 80% (8/10) | 2.1 calls | 7113 tokens | 28.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 27.3s | 2 | 6724 | 96 | 3795 | $0.0000 | Read |
| 2 | ✓ PASS | 36.7s | 2 | 6767 | 139 | 3838 | $0.0000 | Read |
| 3 | ✗ FAIL | 22.9s | 2 | 6704 | 76 | 3775 | $0.0000 | Read |
| 4 | ✓ PASS | 35.1s | 3 | 10581 | 128 | 3910 | $0.0000 | Read, Shell |
| 5 | ✓ PASS | 20.9s | 2 | 6695 | 67 | 3766 | $0.0000 | Read |
| 6 | ✓ PASS | 30.9s | 2 | 6740 | 112 | 3811 | $0.0000 | Read |
| 7 | ✗ FAIL | 26.7s | 2 | 6722 | 94 | 3793 | $0.0000 | Read |
| 8 | ✓ PASS | 32.7s | 2 | 6749 | 121 | 3820 | $0.0000 | Read |
| 9 | ✓ PASS | 30.3s | 2 | 6738 | 110 | 3809 | $0.0000 | Read |
| 10 | ✓ PASS | 24.6s | 2 | 6712 | 84 | 3783 | $0.0000 | Read |

**Failures:**

**Run 3**: output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

**Run 7**: output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'

---

#### R3

**✓ Relative vs Absolute Path** | Correct path resolution | 100% (10/10) | 2.0 calls | 6010 tokens | 11.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 12.1s | 2 | 6013 | 49 | 3086 | $0.0000 | Read |
| 2 | ✓ PASS | 10.6s | 2 | 6008 | 43 | 3081 | $0.0000 | Read |
| 3 | ✓ PASS | 12.5s | 2 | 6011 | 47 | 3084 | $0.0000 | Read |
| 4 | ✓ PASS | 10.8s | 2 | 6008 | 44 | 3081 | $0.0000 | Read |
| 5 | ✓ PASS | 11.3s | 2 | 6011 | 46 | 3084 | $0.0000 | Read |
| 6 | ✓ PASS | 10.6s | 2 | 6008 | 43 | 3081 | $0.0000 | Read |
| 7 | ✓ PASS | 11.9s | 2 | 6013 | 49 | 3086 | $0.0000 | Read |
| 8 | ✓ PASS | 11.3s | 2 | 6011 | 46 | 3084 | $0.0000 | Read |
| 9 | ✓ PASS | 10.8s | 2 | 6008 | 44 | 3081 | $0.0000 | Read |
| 10 | ✓ PASS | 11.3s | 2 | 6011 | 46 | 3084 | $0.0000 | Read |

---

#### S1

**✗ Simple Pattern Search** | Find a specific function definition | 10% (1/10) | 3.3 calls | 10770 tokens | 34.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 40.1s | 3 | 9715 | 185 | 3591 | $0.0000 | Read, Search |
| 2 | ✗ FAIL | 7.6s | 2 | 5926 | 36 | 2997 | $0.0000 | Search |
| 3 | ✗ FAIL | 32.2s | 3 | 9675 | 145 | 3551 | $0.0000 | Read, Search |
| 4 | ✗ FAIL | 47.4s | 4 | 13419 | 213 | 3834 | $0.0000 | Read×2, Search |
| 5 | ✗ FAIL | 11.1s | 2 | 5944 | 54 | 3015 | $0.0000 | Search |
| 6 | ✗ FAIL | 44.0s | 4 | 13308 | 195 | 3777 | $0.0000 | Read×2, Search |
| 7 | ✗ FAIL | 38.4s | 4 | 13375 | 166 | 3789 | $0.0000 | Read×2, Search |
| 8 | ✗ FAIL | 46.8s | 4 | 13319 | 206 | 3789 | $0.0000 | Read×2, Search |
| 9 | ✗ FAIL | 42.5s | 4 | 13318 | 189 | 3778 | $0.0000 | Read×2, Search |
| 10 | ✗ FAIL | 37.0s | 3 | 9699 | 169 | 3580 | $0.0000 | Read, Search |

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

**✗ Multi-Pattern Search** | Find multiple related items | 10% (1/10) | 2.0 calls | 5984 tokens | 15.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 10.1s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 2 | ✗ FAIL | 10.0s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 3 | ✗ FAIL | 9.9s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 4 | ✗ FAIL | 9.9s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 5 | ✗ FAIL | 9.9s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 6 | ✓ PASS | 23.1s | 2 | 6059 | 113 | 3056 | $0.0000 | Shell |
| 7 | ✗ FAIL | 28.7s | 2 | 6117 | 142 | 3085 | $0.0000 | Shell |
| 8 | ✗ FAIL | 33.9s | 2 | 6169 | 168 | 3111 | $0.0000 | Shell |
| 9 | ✗ FAIL | 9.3s | 2 | 5925 | 45 | 2988 | $0.0000 | Shell |
| 10 | ✗ FAIL | 10.5s | 2 | 5931 | 51 | 2994 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain '7'

**Run 2**: output does not contain '7'

**Run 3**: output does not contain '7'

**Run 4**: output does not contain '7'

**Run 5**: output does not contain '7'

**Run 7**: output does not contain '7'

**Run 8**: output does not contain '7'

**Run 9**: output does not contain '7'

**Run 10**: output does not contain '7'

---

#### S3

**✗ Search with File Filtering** | Search in specific file types | 90% (9/10) | 2.0 calls | 6006 tokens | 13.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 12.5s | 2 | 5997 | 60 | 3022 | $0.0000 | Shell |
| 2 | ✓ PASS | 15.3s | 2 | 6015 | 70 | 3030 | $0.0000 | Shell |
| 3 | ✓ PASS | 12.5s | 2 | 5999 | 61 | 3023 | $0.0000 | Shell |
| 4 | ✓ PASS | 15.6s | 2 | 6031 | 77 | 3039 | $0.0000 | Shell |
| 5 | ✓ PASS | 13.7s | 2 | 6009 | 67 | 3027 | $0.0000 | Shell |
| 6 | ✓ PASS | 12.3s | 2 | 5995 | 60 | 3020 | $0.0000 | Shell |
| 7 | ✓ PASS | 12.3s | 2 | 5997 | 60 | 3022 | $0.0000 | Shell |
| 8 | ✗ FAIL | 14.0s | 2 | 6013 | 69 | 3033 | $0.0000 | Shell |
| 9 | ✓ PASS | 13.8s | 2 | 6011 | 68 | 3028 | $0.0000 | Shell |
| 10 | ✓ PASS | 12.2s | 2 | 5995 | 59 | 3021 | $0.0000 | Shell |

**Failures:**

**Run 8**: output contains 'helpers.go' but should not

---

#### S4

**✗ Search Truncation Recovery** | Handle large search results requiring refinement | 30% (3/10) | 4.1 calls | 17496 tokens | 102.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 120.0s | 4 | 18672 | 220 | 7780 | $0.0000 | Read×2, Search |
| 2 | ✓ PASS | 70.0s | 3 | 13076 | 149 | 5110 | $0.0000 | Search, Shell |
| 3 | ✗ FAIL | 120.0s | 5 | 18280 | 402 | 5441 | $0.0000 | Shell×5 |
| 4 | ✗ FAIL | 95.7s | 5 | 16869 | 324 | 4974 | $0.0000 | Search |
| 5 | ✗ FAIL | 38.3s | 3 | 9226 | 167 | 3154 | $0.0000 | Shell×2 |
| 6 | ✗ FAIL | 120.0s | 5 | 23929 | 408 | 5502 | $0.0000 | Search, Shell×3 |
| 7 | ✗ FAIL | 120.0s | 4 | 21486 | 246 | 8193 | $0.0000 | Read×3, Search |
| 8 | ✓ PASS | 120.0s | 4 | 18247 | 354 | 5210 | $0.0000 | Read, Search, Shell |
| 9 | ✓ PASS | 96.0s | 3 | 9288 | 431 | 3244 | $0.0000 | Shell |
| 10 | ✗ FAIL | 120.0s | 5 | 25884 | 380 | 6205 | $0.0000 | Search, Shell×4 |

**Failures:**

**Run 1**: output does not contain 'folder3'

**Run 3**: output does not contain 'folder3'

**Run 4**: output does not contain 'folder3'

**Run 5**: output does not contain 'folder3'

**Run 6**: output does not contain 'folder3'

**Run 7**: output does not contain 'folder3'

**Run 10**: output does not contain 'folder3'

---

#### W1

**✓ Simple File Creation** | Create a new file with content | 100% (10/10) | 2.0 calls | 5954 tokens | 12.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 11.6s | 2 | 5950 | 49 | 3015 | $0.0000 | Write |
| 2 | ✓ PASS | 14.0s | 2 | 5962 | 61 | 3027 | $0.0000 | Write |
| 3 | ✓ PASS | 11.7s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 4 | ✓ PASS | 11.4s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 5 | ✓ PASS | 11.8s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 6 | ✓ PASS | 11.8s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 7 | ✓ PASS | 11.5s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 8 | ✓ PASS | 14.2s | 2 | 5962 | 61 | 3027 | $0.0000 | Write |
| 9 | ✓ PASS | 11.5s | 2 | 5950 | 49 | 3015 | $0.0000 | Write |
| 10 | ✓ PASS | 13.9s | 2 | 5962 | 61 | 3027 | $0.0000 | Write |

---

#### W2

**✓ Multi-line Content** | Write file with multiple lines and formatting | 100% (10/10) | 2.0 calls | 6014 tokens | 18.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 19.6s | 2 | 6017 | 85 | 3055 | $0.0000 | Write |
| 2 | ✓ PASS | 16.8s | 2 | 6005 | 73 | 3043 | $0.0000 | Write |
| 3 | ✓ PASS | 20.1s | 2 | 6021 | 89 | 3059 | $0.0000 | Write |
| 4 | ✓ PASS | 19.5s | 2 | 6017 | 85 | 3055 | $0.0000 | Write |
| 5 | ✓ PASS | 19.4s | 2 | 6018 | 86 | 3056 | $0.0000 | Write |
| 6 | ✓ PASS | 18.3s | 2 | 6012 | 80 | 3050 | $0.0000 | Write |
| 7 | ✓ PASS | 19.4s | 2 | 6017 | 85 | 3055 | $0.0000 | Write |
| 8 | ✓ PASS | 15.5s | 2 | 6000 | 68 | 3038 | $0.0000 | Write |
| 9 | ✓ PASS | 16.8s | 2 | 6005 | 73 | 3043 | $0.0000 | Write |
| 10 | ✓ PASS | 20.7s | 2 | 6024 | 92 | 3062 | $0.0000 | Write |

---

#### W3

**✓ Overwrite Existing** | Handle overwrite of existing file | 100% (10/10) | 3.0 calls | 9141 tokens | 16.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 17.6s | 3 | 9145 | 72 | 3151 | $0.0000 | Write, Write.confirm |
| 2 | ✓ PASS | 15.6s | 3 | 9136 | 63 | 3142 | $0.0000 | Write, Write.confirm |
| 3 | ✓ PASS | 15.6s | 3 | 9136 | 63 | 3142 | $0.0000 | Write, Write.confirm |
| 4 | ✓ PASS | 15.5s | 3 | 9136 | 63 | 3142 | $0.0000 | Write, Write.confirm |
| 5 | ✓ PASS | 20.5s | 3 | 9159 | 86 | 3165 | $0.0000 | Write, Write.confirm |
| 6 | ✓ PASS | 15.6s | 3 | 9136 | 63 | 3142 | $0.0000 | Write, Write.confirm |
| 7 | ✓ PASS | 15.6s | 3 | 9136 | 63 | 3142 | $0.0000 | Write, Write.confirm |
| 8 | ✓ PASS | 17.4s | 3 | 9145 | 72 | 3151 | $0.0000 | Write, Write.confirm |
| 9 | ✓ PASS | 15.2s | 3 | 9134 | 61 | 3140 | $0.0000 | Write, Write.confirm |
| 10 | ✓ PASS | 17.8s | 3 | 9146 | 73 | 3152 | $0.0000 | Write, Write.confirm |

---

#### W4

**✓ Special Characters** | Handle special characters in content | 100% (10/10) | 2.0 calls | 5982 tokens | 12.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 12.3s | 2 | 5982 | 52 | 3029 | $0.0000 | Write |
| 2 | ✓ PASS | 12.3s | 2 | 5983 | 53 | 3030 | $0.0000 | Write |
| 3 | ✓ PASS | 12.1s | 2 | 5982 | 52 | 3029 | $0.0000 | Write |
| 4 | ✓ PASS | 11.7s | 2 | 5980 | 50 | 3027 | $0.0000 | Write |
| 5 | ✓ PASS | 12.3s | 2 | 5983 | 53 | 3030 | $0.0000 | Write |
| 6 | ✓ PASS | 12.1s | 2 | 5982 | 52 | 3029 | $0.0000 | Write |
| 7 | ✓ PASS | 12.3s | 2 | 5983 | 53 | 3030 | $0.0000 | Write |
| 8 | ✓ PASS | 12.4s | 2 | 5983 | 53 | 3030 | $0.0000 | Write |
| 9 | ✓ PASS | 11.7s | 2 | 5980 | 50 | 3027 | $0.0000 | Write |
| 10 | ✓ PASS | 12.2s | 2 | 5982 | 52 | 3029 | $0.0000 | Write |

---

#### W5

**✓ Empty File Creation** | Create an empty file | 100% (10/10) | 2.0 calls | 5944 tokens | 13.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 11.7s | 2 | 5936 | 49 | 3010 | $0.0000 | Write |
| 2 | ✓ PASS | 8.8s | 2 | 5924 | 37 | 2998 | $0.0000 | Write |
| 3 | ✓ PASS | 8.8s | 2 | 5924 | 37 | 2998 | $0.0000 | Write |
| 4 | ✓ PASS | 10.5s | 2 | 5932 | 45 | 3006 | $0.0000 | Write |
| 5 | ✓ PASS | 8.8s | 2 | 5924 | 37 | 2998 | $0.0000 | Write |
| 6 | ✓ PASS | 10.5s | 2 | 5932 | 45 | 3006 | $0.0000 | Write |
| 7 | ✓ PASS | 10.7s | 2 | 5933 | 46 | 3007 | $0.0000 | Write |
| 8 | ✓ PASS | 10.1s | 2 | 5930 | 43 | 3004 | $0.0000 | Write |
| 9 | ✓ PASS | 40.5s | 2 | 6068 | 181 | 3142 | $0.0000 | Write |
| 10 | ✓ PASS | 11.7s | 2 | 5937 | 50 | 3011 | $0.0000 | Write |

---

#### W6

**✓ Path with Spaces** | Handle paths with spaces | 100% (10/10) | 2.0 calls | 5968 tokens | 12.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 12.2s | 2 | 5968 | 52 | 3026 | $0.0000 | Write |
| 2 | ✓ PASS | 12.0s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 3 | ✓ PASS | 11.9s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 4 | ✓ PASS | 11.8s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 5 | ✓ PASS | 12.1s | 2 | 5968 | 52 | 3026 | $0.0000 | Write |
| 6 | ✓ PASS | 11.8s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 7 | ✓ PASS | 12.2s | 2 | 5968 | 52 | 3026 | $0.0000 | Write |
| 8 | ✓ PASS | 12.9s | 2 | 5972 | 56 | 3030 | $0.0000 | Write |
| 9 | ✓ PASS | 11.9s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 10 | ✓ PASS | 11.8s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |

---

## Failure Analysis

| Benchmark | Run | Errors | Last Tool Call |
|-----------|-----|--------|----------------|
| S1 | 2 | output does not contain 'func calculateTotal(it... | Search |
| S1 | 3 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 4 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 5 | output does not contain 'func calculateTotal(it... | Search |
| S1 | 6 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 7 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 8 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 9 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 10 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 1 | output does not contain '7' | Shell |
| S2 | 2 | output does not contain '7' | Shell |
| S2 | 3 | output does not contain '7' | Shell |
| S2 | 4 | output does not contain '7' | Shell |
| S2 | 5 | output does not contain '7' | Shell |
| S2 | 7 | output does not contain '7' | Shell |
| S2 | 8 | output does not contain '7' | Shell |
| S2 | 9 | output does not contain '7' | Shell |
| S2 | 10 | output does not contain '7' | Shell |
| S3 | 8 | output contains 'helpers.go' but should not | Shell |
| S4 | 1 | output does not contain 'folder3' | Read |
| S4 | 3 | output does not contain 'folder3' | Shell |
| S4 | 4 | output does not contain 'folder3' | Search |
| S4 | 5 | output does not contain 'folder3' | Shell |
| S4 | 6 | output does not contain 'folder3' | Shell |
| S4 | 7 | output does not contain 'folder3' | Read |
| S4 | 10 | output does not contain 'folder3' | Shell |
| R1 | 10 | output does not contain 'db_host=localhost' | Read |
| R2 | 3 | output does not contain 'SECTION_BETA'; output ... | Read |
| R2 | 7 | output does not contain 'SECTION_BETA'; output ... | Read |
| E2 | 1 | file file.txt content does not match expected:
... | Edit |
| E2 | 2 | file file.txt content does not match expected:
... | Edit |
| E2 | 3 | file file.txt content does not match expected:
... | Edit |
| E2 | 4 | file file.txt content does not match expected:
... | Edit |
| E2 | 5 | file file.txt content does not match expected:
... | Edit |
| E2 | 6 | file file.txt content does not match expected:
... | Edit |
| E2 | 7 | file file.txt content does not match expected:
... | Edit |
| E2 | 8 | file file.txt content does not match expected:
... | Edit |
| E2 | 9 | file file.txt content does not match expected:
... | Edit |
| E2 | 10 | file file.txt content does not match expected:
... | Edit |
| E3 | 1 | file data.txt content does not match expected:
... | Edit |
| E3 | 2 | file data.txt content does not match expected:
... | Edit |
| E3 | 3 | file data.txt content does not match expected:
... | Edit |
| E3 | 4 | file data.txt content does not match expected:
... | Edit |
| E3 | 5 | file data.txt content does not match expected:
... | Edit |
| E3 | 6 | file data.txt content does not match expected:
... | Edit |
| E3 | 7 | file data.txt content does not match expected:
... | Edit |
| E3 | 8 | file data.txt content does not match expected:
... | Edit |
| E3 | 9 | file data.txt content does not match expected:
... | Edit |
| E3 | 10 | file data.txt content does not match expected:
... | Edit |
| E4 | 1 | file boundary.txt content does not match expect... | Edit |
| E4 | 2 | file boundary.txt content does not match expect... | Edit |
| E4 | 3 | file boundary.txt content does not match expect... | Edit |
| E4 | 4 | file boundary.txt content does not match expect... | Edit |
| E4 | 5 | file boundary.txt content does not match expect... | Edit |
| E4 | 6 | file boundary.txt content does not match expect... | Edit |
| E4 | 7 | file boundary.txt content does not match expect... | Edit |
| E4 | 8 | file boundary.txt content does not match expect... | Edit |
| E4 | 9 | file boundary.txt content does not match expect... | Edit |
| E4 | 10 | file boundary.txt content does not match expect... | Edit |
| E5 | 1 | file code.ts content does not match expected:
-... | Read |
| E5 | 2 | file code.ts content does not match expected:
-... | Edit |
| E5 | 3 | file code.ts content does not match expected:
-... | Edit |
| E5 | 4 | file code.ts content does not match expected:
-... | Edit |
| E5 | 5 | file code.ts content does not match expected:
-... | Edit |
| E5 | 6 | file code.ts content does not match expected:
-... | Edit |
| E5 | 7 | file code.ts content does not match expected:
-... | Edit |
| E5 | 8 | file code.ts content does not match expected:
-... | Edit |
| E5 | 9 | file code.ts content does not match expected:
-... | Edit |
| E5 | 10 | file code.ts content does not match expected:
-... | Edit |
| E6 | 1 | file values.ts content does not match expected:... | Edit |
| E6 | 2 | file values.ts content does not match expected:... | Edit |
| E6 | 3 | file values.ts content does not match expected:... | Edit |
| E6 | 4 | file values.ts content does not match expected:... | Edit |
| E6 | 5 | file values.ts content does not match expected:... | Edit |
| E6 | 7 | file values.ts content does not match expected:... | Edit |
| E6 | 8 | file values.ts content does not match expected:... | Edit |
| E6 | 9 | file values.ts content does not match expected:... | Edit |
| E6 | 10 | file values.ts content does not match expected:... | Edit |
| E7 | 1 | file func.ts content does not match expected:
-... | Edit |
| E7 | 2 | file func.ts content does not match expected:
-... | Edit |
| E7 | 3 | file func.ts content does not match expected:
-... | Edit |
| E7 | 4 | file func.ts content does not match expected:
-... | Edit |
| E7 | 5 | file func.ts content does not match expected:
-... | Edit |
| E7 | 6 | file func.ts content does not match expected:
-... | Edit |
| E7 | 7 | file func.ts content does not match expected:
-... | Edit |
| E7 | 8 | file func.ts content does not match expected:
-... | Edit |
| E7 | 9 | file func.ts content does not match expected:
-... | Edit |
| E7 | 10 | file func.ts content does not match expected:
-... | Edit |
| E8 | 1 | file sample.txt content does not match expected... | Edit |
| E8 | 2 | file sample.txt content does not match expected... | Edit |
| E8 | 3 | file sample.txt content does not match expected... | Edit |
| E8 | 4 | file sample.txt content does not match expected... | Edit |
| E8 | 5 | file sample.txt content does not match expected... | Edit |
| E8 | 6 | output contains 'replaced' but should not; outp... | Edit |
| E8 | 7 | file sample.txt content does not match expected... | Edit |
| E8 | 8 | file sample.txt content does not match expected... | Edit |
| E8 | 9 | file sample.txt content does not match expected... | Edit |
| E8 | 10 | file sample.txt content does not match expected... | Edit |
| E9 | 1 | file numbered.txt content does not match expect... | Edit |
| E9 | 7 | file numbered.txt content does not match expect... | Edit |
| E10 | 1 | file special.txt content does not match expecte... | Edit |
| E10 | 2 | file special.txt content does not match expecte... | Edit |
| E10 | 3 | file special.txt content does not match expecte... | Edit |
| E10 | 4 | file special.txt content does not match expecte... | Edit |
| E10 | 5 | file special.txt content does not match expecte... | Edit |
| E10 | 6 | file special.txt content does not match expecte... | Edit |
| E10 | 7 | file special.txt content does not match expecte... | Edit |
| E10 | 8 | file special.txt content does not match expecte... | Edit |
| E10 | 9 | file special.txt content does not match expecte... | Edit |
| E10 | 10 | file special.txt content does not match expecte... | Edit |
| E11 | 1 | file process.py content does not match expected... | Edit |
| E11 | 2 | file process.py content does not match expected... | Edit |
| E11 | 3 | file process.py content does not match expected... | Shell |
| E11 | 4 | file process.py content does not match expected... | Edit |
| E11 | 5 | file process.py content does not match expected... | Edit |
| E11 | 6 | file process.py content does not match expected... | Edit |
| E11 | 7 | file process.py content does not match expected... | Edit |
| E11 | 8 | file process.py content does not match expected... | Edit |
| E11 | 9 | file process.py content does not match expected... | Edit |
| E11 | 10 | file process.py content does not match expected... | Edit |
| C1 | 3 | output does not contain 'db.example.com' | Read |
| C1 | 6 | output does not contain 'db.example.com' | Read |
| C1 | 8 | output does not contain 'db.example.com' | Read |
| C2 | 1 | file config.yaml content does not match expecte... | Edit |
| C2 | 2 | file config.yaml content does not match expecte... | Edit |
| C2 | 3 | file config.yaml content does not match expecte... | Edit |
| C2 | 5 | file config.yaml content does not match expecte... | Edit |
| C2 | 6 | file config.yaml content does not match expecte... | Edit |
| C2 | 8 | file config.yaml content does not match expecte... | Edit |
| C2 | 10 | file config.yaml content does not match expecte... | Edit |
| C3 | 2 | file utils.ts content does not match expected:
... | Edit |
| C3 | 3 | file utils.ts content does not match expected:
... | Edit |
| C3 | 4 | file utils.ts content does not match expected:
... | Edit |
| C3 | 6 | file utils.ts content does not match expected:
... | Edit |
| C3 | 7 | file utils.ts content does not match expected:
... | Search |
| C3 | 9 | file utils.ts content does not match expected:
... | Search |
| C3 | 10 | file utils.ts content does not match expected:
... | Edit |
| C5 | 2 | output does not contain 'wire.go' | Search |
| C5 | 3 | output does not contain 'wire.go' | Search |
| C5 | 7 | output does not contain 'wire.go' | Search |
| C5 | 8 | output does not contain 'wire.go' | Search |

## Appendix A: Configuration

### Version

```
kvit-coder de939e8 (commit 20260101, built 2026-01-02)
```

### config.yaml

```yaml

```

