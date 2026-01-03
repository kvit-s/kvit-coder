# LLM Tool Usage Benchmark Report

## Metadata

- **Version**: kvit-coder 9604c5e (commit 20260102, built 2026-01-03)
- **Date**: 2026-01-03T02:47:26-06:00
- **Total Benchmarks**: 28
- **Total Runs**: 280

## Summary

| Class | Success Rate | Avg Time/Run |
|-------|--------------|--------------|
| C | 60% (24/40) | 258.4s |
| E | 15% (16/110) | 262.2s |
| R | 100% (30/30) | 57.0s |
| S | 45% (18/40) | 154.9s |
| W | 98% (59/60) | 76.2s |
| **Total** | **52% (147/280)** | **808.6s** |

## Detailed Statistics

### Per-Benchmark Summary

| Benchmark | Success | LLM Calls | Tokens | Generated | Context | Prompt Speed | Gen Speed | Cost | Duration |
|-----------|---------|-----------|--------|-----------|---------|--------------|-----------|------|----------|
| C1 | 90% | 4.1(±0.8) | 13136(±2831) | 172(±51) | 3547 | 693.3 t/s | 6.6 t/s | $0.0000 | 38.1s(±10.5) |
| C2 | 40% | 4.3(±1.2) | 13270(±3493) | 158(±48) | 3331 | 558.1 t/s | 6.2 t/s | $0.0000 | 34.0s(±9.7) |
| C3 | 30% | 7.8(±0.7) | 32270(±4031) | 459(±33) | 5166 | 2363.4 t/s | 15.0 t/s | $0.0000 | 118.4s(±2.8) |
| C5 | 80% | 3.2(±1.5) | 12163(±5881) | 248(±140) | 4973 | 7693.2 t/s | 19.4 t/s | $0.0000 | 67.9s(±36.0) |
| E1 | 90% | 2.0 | 6145(±4) | 75(±7) | 3185 | 278.9 t/s | 5.3 t/s | $0.0000 | 16.6s(±1.3) |
| E2 | 20% | 3.0(±0.4) | 9919(±1206) | 176(±92) | 3675 | 2001.8 t/s | 16.7 t/s | $0.0000 | 40.0s(±19.2) |
| E3 | 0% | 3.2(±0.4) | 10484(±1361) | 126(±27) | 3606 | 516.4 t/s | 6.7 t/s | $0.0000 | 29.5s(±6.3) |
| E4 | 0% | 2.0 | 6106(±13) | 92(±13) | 3150 | 277.0 t/s | 5.3 t/s | $0.0000 | 19.6s(±2.7) |
| E5 | 0% | 2.0 | 6207(±14) | 105(±8) | 3243 | 1425.5 t/s | 7.3 t/s | $0.0000 | 22.8s(±1.5) |
| E6 | 0% | 2.0 | 6071(±13) | 74(±9) | 3119 | 112.9 t/s | 4.9 t/s | $0.0000 | 16.0s(±1.8) |
| E7 | 0% | 2.3(±0.6) | 6982(±1884) | 95(±18) | 3125 | 603.5 t/s | 5.9 t/s | $0.0000 | 20.0s(±3.7) |
| E8 | 0% | 2.0 | 6086(±9) | 89(±9) | 3138 | 527.3 t/s | 5.5 t/s | $0.0000 | 18.9s(±2.0) |
| E9 | 50% | 2.5(±0.5) | 7851(±1718) | 100(±17) | 3304 | 663.0 t/s | 6.3 t/s | $0.0000 | 22.1s(±4.0) |
| E10 | 0% | 2.0 | 6110(±9) | 83(±9) | 3150 | 111.4 t/s | 4.9 t/s | $0.0000 | 17.9s(±1.9) |
| E11 | 0% | 3.2(±2.1) | 10240(±7069) | 181(±144) | 3415 | 1899.1 t/s | 9.2 t/s | $0.0000 | 38.9s(±30.6) |
| R1 | 100% | 2.0 | 6043(±2) | 62(±2) | 3122 | 662.9 t/s | 5.7 t/s | $0.0000 | 17.8s(±6.0) |
| R2 | 100% | 2.2(±0.4) | 7499(±1541) | 106(±18) | 3821 | 3655.4 t/s | 16.7 t/s | $0.0000 | 27.3s(±4.1) |
| R3 | 100% | 2.0 | 6018(±16) | 53(±17) | 3091 | 299.9 t/s | 5.7 t/s | $0.0000 | 11.9s(±3.2) |
| S1 | 50% | 3.3(±0.6) | 10803(±2372) | 186(±55) | 3627 | 1899.7 t/s | 11.4 t/s | $0.0000 | 42.0s(±12.8) |
| S2 | 0% | 2.0 | 5939(±28) | 54(±14) | 2997 | 341.4 t/s | 5.9 t/s | $0.0000 | 11.5s(±3.1) |
| S3 | 100% | 2.3(±0.5) | 6929(±1404) | 86(±30) | 3040 | 1266.0 t/s | 8.4 t/s | $0.0000 | 17.9s(±6.1) |
| S4 | 30% | 4.0(±2.0) | 17693(±10994) | 249(±137) | 5259 | 6338.2 t/s | 18.3 t/s | $0.0000 | 83.5s(±43.9) |
| W1 | 100% | 2.0 | 5954(±5) | 53(±5) | 3019 | 93.5 t/s | 5.0 t/s | $0.0000 | 11.3s(±1.0) |
| W2 | 100% | 2.0 | 6011(±9) | 81(±8) | 3051 | 105.3 t/s | 4.9 t/s | $0.0000 | 16.7s(±1.7) |
| W3 | 90% | 2.9(±0.3) | 8836(±942) | 70(±11) | 3143 | 96.2 t/s | 5.0 t/s | $0.0000 | 15.8s(±2.4) |
| W4 | 100% | 2.0 | 5988(±9) | 58(±9) | 3035 | 101.7 t/s | 5.0 t/s | $0.0000 | 12.3s(±1.8) |
| W5 | 100% | 2.0 | 5930(±5) | 43(±5) | 3004 | 84.8 t/s | 5.0 t/s | $0.0000 | 9.2s(±1.1) |
| W6 | 100% | 2.0 | 5967(±2) | 51(±2) | 3025 | 99.6 t/s | 5.0 t/s | $0.0000 | 10.9s(±0.7) |

### Per-Benchmark Details

#### C1

**✗ Search Then Read** | Find and read a file | 90% (9/10) | 4.1 calls | 13136 tokens | 38.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 59.3s | 6 | 19858 | 278 | 3891 | $0.0000 | Read×2, Search×3 |
| 2 | ✓ PASS | 48.4s | 4 | 13260 | 216 | 3727 | $0.0000 | Read×2, Search |
| 3 | ✗ FAIL | 42.3s | 4 | 13204 | 187 | 3692 | $0.0000 | Read×2, Search |
| 4 | ✓ PASS | 29.8s | 4 | 12501 | 131 | 3429 | $0.0000 | Read×2, Search |
| 5 | ✓ PASS | 32.0s | 3 | 9567 | 144 | 3474 | $0.0000 | Read, Search |
| 6 | ✓ PASS | 47.4s | 5 | 15947 | 221 | 3557 | $0.0000 | Read×2, Search×2 |
| 7 | ✓ PASS | 36.4s | 4 | 12516 | 164 | 3420 | $0.0000 | Read, Search, Shell |
| 8 | ✓ PASS | 29.2s | 4 | 12498 | 128 | 3426 | $0.0000 | Read×2, Search |
| 9 | ✓ PASS | 22.8s | 4 | 12425 | 96 | 3376 | $0.0000 | Read×2, Search |
| 10 | ✓ PASS | 33.3s | 3 | 9587 | 152 | 3480 | $0.0000 | Read, Search |

**Failures:**

**Run 3**: output does not contain 'db.example.com'

---

#### C2

**✗ Read-Modify-Write** | Complete edit workflow | 40% (4/10) | 4.3 calls | 13270 tokens | 34.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 25.8s | 3 | 9481 | 119 | 3397 | $0.0000 | Edit, Read |
| 2 | ✓ PASS | 23.5s | 3 | 9392 | 104 | 3324 | $0.0000 | Edit, Read |
| 3 | ✓ PASS | 33.6s | 5 | 15316 | 157 | 3245 | $0.0000 | Shell, Write, Write.confirm |
| 4 | ✗ FAIL | 36.9s | 4 | 12437 | 169 | 3396 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 50.3s | 6 | 18229 | 235 | 3313 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 25.5s | 3 | 9489 | 112 | 3413 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 22.0s | 3 | 9374 | 98 | 3312 | $0.0000 | Edit, Read |
| 8 | ✓ PASS | 43.1s | 6 | 18266 | 205 | 3253 | $0.0000 | Shell, Write, Write.confirm |
| 9 | ✗ FAIL | 47.7s | 5 | 15403 | 229 | 3415 | $0.0000 | Edit, Read |
| 10 | ✓ PASS | 31.8s | 5 | 15309 | 150 | 3239 | $0.0000 | Shell, Write, Write.confirm |

**Failures:**

**Run 1**: 
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


**Run 4**: 
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
@@ -1,4 +1,6 @@
 server:
   host: localhost
-  port: 8080
+   1│server:
+   2│  host: localhost
+   3│  port: 8080
 

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
+    port: 8080
 

```


**Run 9**: 
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


---

#### C3

**✗ Search-Read-Edit** | Find, understand, and modify | 30% (3/10) | 7.8 calls | 32270 tokens | 118.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 120.0s | 7 | 27991 | 428 | 4882 | $0.0000 | Edit×3, Read×3, Search |
| 2 | ✗ FAIL | 112.4s | 8 | 33252 | 477 | 5249 | $0.0000 | Edit×3, Read×3, Search |
| 3 | ✓ PASS | 113.7s | 8 | 33222 | 492 | 5227 | $0.0000 | Edit×3, Read×3, Search |
| 4 | ✗ FAIL | 120.0s | 9 | 38757 | 471 | 5558 | $0.0000 | Edit×3, Read×3, Search×2 |
| 5 | ✓ PASS | 120.0s | 7 | 27989 | 426 | 4883 | $0.0000 | Edit×3, Read×3, Search |
| 6 | ✗ FAIL | 120.0s | 9 | 38805 | 470 | 5568 | $0.0000 | Edit×3, Read×3, Search×2 |
| 7 | ✗ FAIL | 120.0s | 7 | 27962 | 405 | 4865 | $0.0000 | Edit×3, Read×3, Search |
| 8 | ✗ FAIL | 120.0s | 8 | 33425 | 476 | 5232 | $0.0000 | Edit×4, Read×3, Search |
| 9 | ✗ FAIL | 120.0s | 7 | 28000 | 431 | 4887 | $0.0000 | Edit×3, Read×3, Search |
| 10 | ✗ FAIL | 118.0s | 8 | 33302 | 514 | 5304 | $0.0000 | Edit×3, Read×3, Search |

**Failures:**

**Run 2**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -4,4 +4,6 @@
 
 function deprecatedFunc() {}
 function newFunc() {}
+function deprecatedFunc() {}
+function newFunc() {}
 

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
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,5 @@
 function process() {
     newFunc();
 }
+}
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,5 @@
 function handle() {
     newFunc();
 }
+}
 

```


```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -1,5 +1,6 @@
 function helper() {
     newFunc();
+}
 }
 
 function deprecatedFunc() {}

```


**Run 8**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+function process() {
 function process() {
     newFunc();
 }

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+function handle() {
 function handle() {
     newFunc();
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

**✗ Needle in Haystack Search** | Find specific initialization among many usages | 80% (8/10) | 3.2 calls | 12163 tokens | 67.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 120.0s | 3 | 11458 | 340 | 5422 | $0.0000 | Read, Search×2 |
| 2 | ✗ FAIL | 30.5s | 2 | 7044 | 112 | 4118 | $0.0000 | Search |
| 3 | ✓ PASS | 34.8s | 2 | 8051 | 94 | 5122 | $0.0000 | Search |
| 4 | ✓ PASS | 88.8s | 3 | 13825 | 339 | 5645 | $0.0000 | Read, Search |
| 5 | ✗ FAIL | 16.3s | 2 | 5980 | 77 | 3037 | $0.0000 | Search |
| 6 | ✓ PASS | 71.9s | 2 | 8225 | 268 | 5296 | $0.0000 | Search |
| 7 | ✓ PASS | 39.1s | 2 | 8067 | 110 | 5138 | $0.0000 | Search |
| 8 | ✓ PASS | 120.0s | 6 | 21655 | 503 | 5737 | $0.0000 | Read, Search×5 |
| 9 | ✓ PASS | 58.0s | 4 | 13451 | 242 | 4354 | $0.0000 | Search×3 |
| 10 | ✓ PASS | 99.1s | 6 | 23878 | 397 | 5857 | $0.0000 | Read, Search×4 |

**Failures:**

**Run 2**: output does not contain 'wire.go'

**Run 5**: output does not contain 'wire.go'

---

#### E1

**✗ Single Line Replace** | Replace a single line | 90% (9/10) | 2.0 calls | 6145 tokens | 16.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 15.5s | 2 | 6145 | 73 | 3183 | $0.0000 | Edit |
| 2 | ✓ PASS | 15.6s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 3 | ✓ PASS | 16.3s | 2 | 6145 | 73 | 3183 | $0.0000 | Edit |
| 4 | ✓ PASS | 16.2s | 2 | 6145 | 73 | 3183 | $0.0000 | Edit |
| 5 | ✓ PASS | 15.8s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 6 | ✓ PASS | 15.6s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 7 | ✓ PASS | 17.2s | 2 | 6149 | 79 | 3189 | $0.0000 | Edit |
| 8 | ✗ FAIL | 20.1s | 2 | 6152 | 93 | 3200 | $0.0000 | Edit |
| 9 | ✓ PASS | 16.2s | 2 | 6141 | 71 | 3181 | $0.0000 | Edit |
| 10 | ✓ PASS | 17.4s | 2 | 6151 | 79 | 3189 | $0.0000 | Edit |

**Failures:**

**Run 8**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -3,6 +3,7 @@
 FUNCTION C
 FUNCTION D
 NEW LINE E
+FUNCTION E
 FUNCTION F
 FUNCTION G
 FUNCTION H

```


---

#### E2

**✗ Multi-line Insert** | Insert multiple lines at position | 20% (2/10) | 3.0 calls | 9919 tokens | 40.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 45.4s | 3 | 9994 | 213 | 3730 | $0.0000 | Edit, Read |
| 2 | ✗ FAIL | 48.8s | 4 | 12845 | 220 | 3614 | $0.0000 | Edit, Read |
| 3 | ✓ PASS | 29.0s | 3 | 9817 | 123 | 3586 | $0.0000 | Edit, Read |
| 4 | ✓ PASS | 21.5s | 3 | 9740 | 86 | 3533 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 29.3s | 3 | 9848 | 124 | 3603 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 91.5s | 2 | 7494 | 420 | 4210 | $0.0000 | Edit |
| 7 | ✗ FAIL | 30.1s | 3 | 9802 | 128 | 3568 | $0.0000 | Edit, Read |
| 8 | ✗ FAIL | 25.1s | 3 | 9766 | 104 | 3542 | $0.0000 | Edit, Read |
| 9 | ✗ FAIL | 43.3s | 3 | 9994 | 191 | 3724 | $0.0000 | Edit, Read |
| 10 | ✗ FAIL | 35.7s | 3 | 9892 | 151 | 3644 | $0.0000 | Edit, Read |

**Failures:**

**Run 1**: 
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
+            INSERTED
+            INSERTED
+            INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 2**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,10 +7,9 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
 INSERTED
-INSERTED
-INSERTED
+        INSERTED
+        INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 5**: 
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


**Run 6**: 
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
+                                                                                                        INSERTED
+                                                                                                                INSERTED
+                                                                                                                INSERTED
 FUNCTION O
 FUNCTION P
 FUNCTION Q

```


**Run 7**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,7 +7,6 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
 INSERTED
 INSERTED
 INSERTED

```


**Run 8**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,10 +7,10 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
 INSERTED
 INSERTED
 INSERTED
+FUNCTION J
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 9**: 
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
+                INSERTED
+                INSERTED
+                INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 10**: 
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


---

#### E3

**✗ Delete Lines** | Delete a range of lines | 0% (0/10) | 3.2 calls | 10484 tokens | 29.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 25.5s | 3 | 9750 | 114 | 3530 | $0.0000 | Edit, Read |
| 2 | ✗ FAIL | 25.4s | 3 | 9738 | 108 | 3526 | $0.0000 | Edit, Read |
| 3 | ✗ FAIL | 25.4s | 3 | 9738 | 108 | 3525 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 45.2s | 3 | 10216 | 193 | 3912 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 26.6s | 3 | 9790 | 112 | 3579 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 25.9s | 3 | 9740 | 110 | 3527 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 36.0s | 4 | 13238 | 153 | 3719 | $0.0000 | Edit, Read×2 |
| 8 | ✗ FAIL | 33.4s | 4 | 13145 | 141 | 3680 | $0.0000 | Edit, Read×2 |
| 9 | ✗ FAIL | 26.4s | 3 | 9742 | 112 | 3530 | $0.0000 | Edit, Read |
| 10 | ✗ FAIL | 25.5s | 3 | 9739 | 109 | 3527 | $0.0000 | Edit, Read |

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
@@ -8,6 +8,15 @@
 function H
 function I
 function J
+   11│function J
+   12│function L
+   13│function M
+   14│function O
+   15│function P
+   16│function Q
+   17│function R
+   18│function S
+   19│function T
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
+
+
+
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
@@ -8,6 +8,7 @@
 function H
 function I
 function J
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
@@ -8,6 +8,7 @@
 function H
 function I
 function J
+
 function O
 function P
 function Q

```


---

#### E4

**✗ Boundary Edit** | Test boundary conditions | 0% (0/10) | 2.0 calls | 6106 tokens | 19.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 15.6s | 2 | 6089 | 75 | 3133 | $0.0000 | Edit |
| 2 | ✗ FAIL | 19.7s | 2 | 6106 | 92 | 3150 | $0.0000 | Edit |
| 3 | ✗ FAIL | 22.4s | 2 | 6119 | 105 | 3163 | $0.0000 | Edit |
| 4 | ✗ FAIL | 24.9s | 2 | 6132 | 118 | 3176 | $0.0000 | Edit |
| 5 | ✗ FAIL | 16.7s | 2 | 6091 | 77 | 3135 | $0.0000 | Edit |
| 6 | ✗ FAIL | 18.8s | 2 | 6102 | 88 | 3146 | $0.0000 | Edit |
| 7 | ✗ FAIL | 21.9s | 2 | 6116 | 102 | 3160 | $0.0000 | Edit |
| 8 | ✗ FAIL | 19.4s | 2 | 6104 | 90 | 3148 | $0.0000 | Edit |
| 9 | ✗ FAIL | 16.7s | 2 | 6089 | 75 | 3133 | $0.0000 | Edit |
| 10 | ✗ FAIL | 20.1s | 2 | 6108 | 94 | 3152 | $0.0000 | Edit |

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

**✗ Replace All Occurrences** | Replace multiple occurrences | 0% (0/10) | 2.0 calls | 6207 tokens | 22.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 22.0s | 2 | 6213 | 106 | 3248 | $0.0000 | Edit |
| 2 | ✗ FAIL | 22.1s | 2 | 6206 | 99 | 3241 | $0.0000 | Edit |
| 3 | ✗ FAIL | 23.8s | 2 | 6216 | 109 | 3251 | $0.0000 | Edit |
| 4 | ✗ FAIL | 21.6s | 2 | 6205 | 98 | 3240 | $0.0000 | Edit |
| 5 | ✗ FAIL | 24.4s | 2 | 6220 | 113 | 3255 | $0.0000 | Edit |
| 6 | ✗ FAIL | 25.2s | 2 | 6231 | 118 | 3264 | $0.0000 | Edit |
| 7 | ✗ FAIL | 21.4s | 2 | 6203 | 96 | 3238 | $0.0000 | Edit |
| 8 | ✗ FAIL | 24.9s | 2 | 6173 | 117 | 3218 | $0.0000 | Edit |
| 9 | ✗ FAIL | 21.0s | 2 | 6203 | 96 | 3238 | $0.0000 | Edit |
| 10 | ✗ FAIL | 21.2s | 2 | 6204 | 97 | 3239 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,14 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+  // function implementation
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


**Run 2**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,14 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+  // function implementation
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
+  // function implementation
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
@@ -1,12 +1,14 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+  // function implementation
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


**Run 5**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,14 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+  // function implementation
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


**Run 6**: 
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
+  // function implementation
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
+  // function implementation
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

**✗ Context-Specific Replace** | Replace with context for uniqueness | 0% (0/10) | 2.0 calls | 6071 tokens | 16.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 15.1s | 2 | 6063 | 73 | 3117 | $0.0000 | Edit |
| 2 | ✗ FAIL | 17.0s | 2 | 6076 | 80 | 3122 | $0.0000 | Edit |
| 3 | ✗ FAIL | 15.6s | 2 | 6065 | 73 | 3118 | $0.0000 | Edit |
| 4 | ✗ FAIL | 20.5s | 2 | 6093 | 97 | 3139 | $0.0000 | Edit |
| 5 | ✗ FAIL | 15.2s | 2 | 6070 | 70 | 3114 | $0.0000 | Edit |
| 6 | ✗ FAIL | 16.4s | 2 | 6092 | 75 | 3131 | $0.0000 | Edit |
| 7 | ✗ FAIL | 15.7s | 2 | 6062 | 72 | 3116 | $0.0000 | Edit |
| 8 | ✗ FAIL | 13.3s | 2 | 6050 | 60 | 3104 | $0.0000 | Edit |
| 9 | ✗ FAIL | 15.0s | 2 | 6064 | 68 | 3110 | $0.0000 | Edit |
| 10 | ✗ FAIL | 16.0s | 2 | 6071 | 75 | 3117 | $0.0000 | Edit |

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

**✗ Multi-line Block Replace** | Replace multi-line block | 0% (0/10) | 2.3 calls | 6982 tokens | 20.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 17.2s | 2 | 6097 | 83 | 3135 | $0.0000 | Edit |
| 2 | ✗ FAIL | 19.0s | 2 | 6099 | 89 | 3139 | $0.0000 | Edit |
| 3 | ✗ FAIL | 17.5s | 4 | 11972 | 84 | 3014 | $0.0000 | Read×2 |
| 4 | ✗ FAIL | 18.6s | 2 | 6097 | 87 | 3137 | $0.0000 | Edit |
| 5 | ✗ FAIL | 29.7s | 3 | 9066 | 143 | 3144 | $0.0000 | Edit |
| 6 | ✗ FAIL | 18.6s | 2 | 6102 | 88 | 3140 | $0.0000 | Edit |
| 7 | ✗ FAIL | 19.9s | 2 | 6102 | 92 | 3142 | $0.0000 | Edit |
| 8 | ✗ FAIL | 23.4s | 2 | 6124 | 110 | 3162 | $0.0000 | Edit |
| 9 | ✗ FAIL | 16.6s | 2 | 6087 | 77 | 3127 | $0.0000 | Edit |
| 10 | ✗ FAIL | 19.8s | 2 | 6074 | 94 | 3114 | $0.0000 | Edit |

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

**✗ No Match Handling** | Handle search text not found | 0% (0/10) | 2.0 calls | 6086 tokens | 18.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 14.9s | 2 | 6069 | 72 | 3117 | $0.0000 | Edit |
| 2 | ✗ FAIL | 19.6s | 2 | 6089 | 92 | 3137 | $0.0000 | Edit |
| 3 | ✗ FAIL | 19.1s | 2 | 6088 | 90 | 3144 | $0.0000 | Edit |
| 4 | ✗ FAIL | 19.0s | 2 | 6087 | 89 | 3143 | $0.0000 | Edit |
| 5 | ✗ FAIL | 21.7s | 2 | 6100 | 103 | 3148 | $0.0000 | Edit |
| 6 | ✗ FAIL | 20.8s | 2 | 6095 | 98 | 3143 | $0.0000 | Edit |
| 7 | ✗ FAIL | 18.7s | 2 | 6085 | 87 | 3141 | $0.0000 | Edit |
| 8 | ✗ FAIL | 20.6s | 2 | 6094 | 97 | 3142 | $0.0000 | Edit |
| 9 | ✗ FAIL | 18.9s | 2 | 6085 | 87 | 3141 | $0.0000 | Edit |
| 10 | ✗ FAIL | 15.8s | 2 | 6072 | 74 | 3128 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-This is a sample file.
+replacement
 It has multiple lines.
 Nothing special here.
 

```

output contains 'successfully' but should not

**Run 2**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-This is a sample file.
+replacement
 It has multiple lines.
 Nothing special here.
 

```

output contains 'successfully' but should not

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

output contains 'successfully' but should not

**Run 5**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-This is a sample file.
+targetString
 It has multiple lines.
 Nothing special here.
 

```

output contains 'successfully' but should not

**Run 6**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-This is a sample file.
+targetString
 It has multiple lines.
 Nothing special here.
 

```

output contains 'replaced' but should not

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


**Run 8**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-This is a sample file.
+targetString
 It has multiple lines.
 Nothing special here.
 

```


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

output contains 'replaced' but should not
output contains 'successfully' but should not

---

#### E9

**✗ Empty Content** | Handle empty replacement | 50% (5/10) | 2.5 calls | 7851 tokens | 22.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 21.0s | 2 | 6153 | 103 | 3191 | $0.0000 | Edit |
| 2 | ✓ PASS | 27.9s | 3 | 9582 | 124 | 3446 | $0.0000 | Edit, Read |
| 3 | ✓ PASS | 23.5s | 3 | 9561 | 103 | 3425 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 21.0s | 2 | 6148 | 98 | 3186 | $0.0000 | Edit |
| 5 | ✓ PASS | 26.8s | 3 | 9578 | 120 | 3442 | $0.0000 | Edit, Read |
| 6 | ✓ PASS | 23.0s | 3 | 9558 | 100 | 3422 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 13.9s | 2 | 6098 | 61 | 3146 | $0.0000 | Edit |
| 8 | ✗ FAIL | 21.4s | 2 | 6137 | 100 | 3185 | $0.0000 | Edit |
| 9 | ✗ FAIL | 17.3s | 2 | 6130 | 80 | 3168 | $0.0000 | Edit |
| 10 | ✓ PASS | 25.1s | 3 | 9569 | 111 | 3433 | $0.0000 | Edit, Read |

**Failures:**

**Run 1**: 
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


**Run 4**: 
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


**Run 7**: 
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


**Run 8**: 
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


**Run 9**: 
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

**✗ Special Characters** | Handle special characters in replacement | 0% (0/10) | 2.0 calls | 6110 tokens | 17.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 17.3s | 2 | 6111 | 84 | 3151 | $0.0000 | Edit |
| 2 | ✗ FAIL | 18.3s | 2 | 6111 | 84 | 3151 | $0.0000 | Edit |
| 3 | ✗ FAIL | 18.0s | 2 | 6109 | 82 | 3149 | $0.0000 | Edit |
| 4 | ✗ FAIL | 20.6s | 2 | 6123 | 96 | 3163 | $0.0000 | Edit |
| 5 | ✗ FAIL | 19.5s | 2 | 6118 | 91 | 3158 | $0.0000 | Edit |
| 6 | ✗ FAIL | 13.2s | 2 | 6088 | 61 | 3128 | $0.0000 | Edit |
| 7 | ✗ FAIL | 19.3s | 2 | 6116 | 89 | 3156 | $0.0000 | Edit |
| 8 | ✗ FAIL | 17.9s | 2 | 6109 | 82 | 3149 | $0.0000 | Edit |
| 9 | ✗ FAIL | 17.4s | 2 | 6105 | 78 | 3145 | $0.0000 | Edit |
| 10 | ✗ FAIL | 17.1s | 2 | 6106 | 79 | 3146 | $0.0000 | Edit |

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
@@ -1,6 +1,7 @@
+value = $100 + 50%
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

**✗ Indentation Preservation** | Maintain correct indentation (critical for Python) | 0% (0/10) | 3.2 calls | 10240 tokens | 38.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 39.0s | 4 | 12166 | 197 | 3240 | $0.0000 | Edit |
| 2 | ✗ FAIL | 20.4s | 2 | 6208 | 91 | 3232 | $0.0000 | Edit |
| 3 | ✗ FAIL | 115.0s | 9 | 29845 | 540 | 4261 | $0.0000 | Edit×3, Read×2 |
| 4 | ✗ FAIL | 19.2s | 2 | 6205 | 86 | 3229 | $0.0000 | Edit |
| 5 | ✗ FAIL | 21.1s | 2 | 6215 | 96 | 3239 | $0.0000 | Edit |
| 6 | ✗ FAIL | 24.1s | 2 | 6247 | 110 | 3264 | $0.0000 | Edit |
| 7 | ✗ FAIL | 77.6s | 4 | 13895 | 354 | 3963 | $0.0000 | Edit×3 |
| 8 | ✗ FAIL | 19.9s | 2 | 6213 | 89 | 3234 | $0.0000 | Edit |
| 9 | ✗ FAIL | 33.4s | 3 | 9208 | 158 | 3260 | $0.0000 | Edit |
| 10 | ✗ FAIL | 19.1s | 2 | 6201 | 86 | 3225 | $0.0000 | Edit |

**Failures:**

**Run 1**: 
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

**Run 2**: 
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


**Run 3**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -1,7 +1,8 @@
 def outer():
     if True:
-        process_data()
+process_data()
         validate_input()
         transform_data()
+        # placeholder
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: expected an indented block after 'if' statement on line 2 (process.py, line 3)

**Run 4**: 
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

**Run 5**: 
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

**Run 6**: 
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
+    validate_input()
+    transform_data()
+        # placeholder
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: expected an indented block after 'if' statement on line 2 (process.py, line 3)

**Run 7**: 
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
+    validate_input()
+    transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 7)

**Run 8**: 
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
+    transform_data()

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 7)

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
-        finalize()
+        # placeholder
+validate_input()
+transform_data()
 

```


---

#### R1

**✓ Simple File Read** | Read an entire small file | 100% (10/10) | 2.0 calls | 6043 tokens | 17.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 26.1s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 2 | ✓ PASS | 14.0s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 3 | ✓ PASS | 25.4s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 4 | ✓ PASS | 15.1s | 2 | 6049 | 68 | 3128 | $0.0000 | Read |
| 5 | ✓ PASS | 13.9s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 6 | ✓ PASS | 13.3s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 7 | ✓ PASS | 13.6s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 8 | ✓ PASS | 29.1s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 9 | ✓ PASS | 13.5s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |
| 10 | ✓ PASS | 13.8s | 2 | 6042 | 61 | 3121 | $0.0000 | Read |

---

#### R2

**✓ Truncation Recovery** | Handle truncated output by chunked reading | 100% (10/10) | 2.2 calls | 7499 tokens | 27.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 21.3s | 2 | 6711 | 83 | 3782 | $0.0000 | Read |
| 2 | ✓ PASS | 29.6s | 2 | 6748 | 120 | 3819 | $0.0000 | Read |
| 3 | ✓ PASS | 28.5s | 2 | 6735 | 107 | 3806 | $0.0000 | Read |
| 4 | ✓ PASS | 31.2s | 2 | 6755 | 127 | 3826 | $0.0000 | Read |
| 5 | ✓ PASS | 33.7s | 3 | 10581 | 128 | 3909 | $0.0000 | Read, Shell |
| 6 | ✓ PASS | 31.8s | 3 | 10581 | 128 | 3909 | $0.0000 | Read, Shell |
| 7 | ✓ PASS | 22.8s | 2 | 6712 | 84 | 3783 | $0.0000 | Read |
| 8 | ✓ PASS | 25.1s | 2 | 6724 | 96 | 3795 | $0.0000 | Read |
| 9 | ✓ PASS | 22.8s | 2 | 6712 | 84 | 3783 | $0.0000 | Read |
| 10 | ✓ PASS | 26.2s | 2 | 6731 | 103 | 3802 | $0.0000 | Read |

---

#### R3

**✓ Relative vs Absolute Path** | Correct path resolution | 100% (10/10) | 2.0 calls | 6018 tokens | 11.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 9.9s | 2 | 6011 | 46 | 3084 | $0.0000 | Read |
| 2 | ✓ PASS | 16.9s | 2 | 6044 | 79 | 3117 | $0.0000 | Read |
| 3 | ✓ PASS | 9.8s | 2 | 6008 | 43 | 3081 | $0.0000 | Read |
| 4 | ✓ PASS | 9.7s | 2 | 6007 | 42 | 3080 | $0.0000 | Read |
| 5 | ✓ PASS | 10.7s | 2 | 6011 | 47 | 3084 | $0.0000 | Read |
| 6 | ✓ PASS | 10.4s | 2 | 6011 | 46 | 3084 | $0.0000 | Read |
| 7 | ✓ PASS | 10.7s | 2 | 6011 | 47 | 3084 | $0.0000 | Read |
| 8 | ✓ PASS | 10.8s | 2 | 6008 | 43 | 3081 | $0.0000 | Read |
| 9 | ✓ PASS | 10.5s | 2 | 6011 | 47 | 3084 | $0.0000 | Read |
| 10 | ✓ PASS | 19.3s | 2 | 6056 | 92 | 3129 | $0.0000 | Read |

---

#### S1

**✗ Simple Pattern Search** | Find a specific function definition | 50% (5/10) | 3.3 calls | 10803 tokens | 42.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 31.8s | 3 | 9678 | 148 | 3553 | $0.0000 | Read, Search |
| 2 | ✓ PASS | 42.9s | 3 | 9715 | 185 | 3596 | $0.0000 | Read, Search |
| 3 | ✓ PASS | 42.4s | 3 | 9723 | 193 | 3599 | $0.0000 | Read, Search |
| 4 | ✗ FAIL | 47.9s | 4 | 13416 | 210 | 3831 | $0.0000 | Read×2, Search |
| 5 | ✓ PASS | 58.2s | 4 | 13459 | 253 | 3874 | $0.0000 | Read×2, Search |
| 6 | ✗ FAIL | 13.6s | 2 | 5955 | 65 | 3026 | $0.0000 | Search |
| 7 | ✓ PASS | 61.2s | 4 | 13387 | 277 | 3861 | $0.0000 | Read×2, Search |
| 8 | ✓ PASS | 41.8s | 3 | 9720 | 190 | 3596 | $0.0000 | Read, Search |
| 9 | ✗ FAIL | 45.1s | 4 | 13288 | 186 | 3766 | $0.0000 | Read×2, Search |
| 10 | ✗ FAIL | 35.1s | 3 | 9688 | 158 | 3564 | $0.0000 | Read, Search |

**Failures:**

**Run 1**: output does not contain 'func calculateTotal(items []int)'

**Run 4**: output does not contain 'func calculateTotal(items []int)'

**Run 6**: output does not contain 'func calculateTotal(items []int)'

**Run 9**: output does not contain 'func calculateTotal(items []int)'

**Run 10**: output does not contain 'func calculateTotal(items []int)'

---

#### S2

**✗ Multi-Pattern Search** | Find multiple related items | 0% (0/10) | 2.0 calls | 5939 tokens | 11.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 9.8s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 2 | ✗ FAIL | 11.0s | 2 | 5931 | 51 | 2994 | $0.0000 | Shell |
| 3 | ✗ FAIL | 10.3s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 4 | ✗ FAIL | 10.1s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 5 | ✗ FAIL | 20.2s | 2 | 6021 | 94 | 3037 | $0.0000 | Shell |
| 6 | ✗ FAIL | 9.6s | 2 | 5925 | 45 | 2988 | $0.0000 | Shell |
| 7 | ✗ FAIL | 10.3s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 8 | ✗ FAIL | 9.9s | 2 | 5928 | 48 | 2991 | $0.0000 | Shell |
| 9 | ✗ FAIL | 9.8s | 2 | 5925 | 45 | 2988 | $0.0000 | Shell |
| 10 | ✗ FAIL | 13.7s | 2 | 5946 | 66 | 3009 | $0.0000 | Shell |

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

**✓ Search with File Filtering** | Search in specific file types | 100% (10/10) | 2.3 calls | 6929 tokens | 17.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 18.5s | 2 | 6061 | 92 | 3054 | $0.0000 | Shell |
| 2 | ✓ PASS | 13.3s | 2 | 6003 | 63 | 3025 | $0.0000 | Shell |
| 3 | ✓ PASS | 13.2s | 2 | 5999 | 62 | 3022 | $0.0000 | Shell |
| 4 | ✓ PASS | 15.0s | 2 | 6013 | 69 | 3029 | $0.0000 | Shell |
| 5 | ✓ PASS | 12.9s | 2 | 5997 | 60 | 3022 | $0.0000 | Shell |
| 6 | ✓ PASS | 25.3s | 3 | 8984 | 124 | 3026 | $0.0000 | Shell |
| 7 | ✓ PASS | 27.2s | 3 | 9236 | 128 | 3148 | $0.0000 | Shell×2 |
| 8 | ✓ PASS | 27.9s | 3 | 8994 | 137 | 3023 | $0.0000 | Shell |
| 9 | ✓ PASS | 13.2s | 2 | 6001 | 62 | 3024 | $0.0000 | Shell |
| 10 | ✓ PASS | 12.8s | 2 | 5999 | 61 | 3023 | $0.0000 | Shell |

---

#### S4

**✗ Search Truncation Recovery** | Handle large search results requiring refinement | 30% (3/10) | 4.0 calls | 17693 tokens | 83.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 120.0s | 2 | 8105 | 272 | 5182 | $0.0000 | Read, Search |
| 2 | ✗ FAIL | 120.0s | 6 | 18444 | 513 | 3312 | $0.0000 | Shell×3 |
| 3 | ✗ FAIL | 120.0s | 4 | 21499 | 249 | 8195 | $0.0000 | Read×3, Search |
| 4 | ✓ PASS | 76.6s | 4 | 16162 | 293 | 5191 | $0.0000 | Search, Shell |
| 5 | ✗ FAIL | 120.0s | 6 | 29919 | 320 | 5633 | $0.0000 | Read, Search, Shell×4 |
| 6 | ✗ FAIL | 29.6s | 2 | 7906 | 73 | 4983 | $0.0000 | Search |
| 7 | ✓ PASS | 19.0s | 2 | 6006 | 91 | 3036 | $0.0000 | Shell |
| 8 | ✗ FAIL | 120.0s | 4 | 21501 | 256 | 8198 | $0.0000 | Read×3, Search |
| 9 | ✗ FAIL | 10.7s | 2 | 5932 | 50 | 2995 | $0.0000 | Shell |
| 10 | ✓ PASS | 98.6s | 8 | 41458 | 375 | 5861 | $0.0000 | Read×2, Search, Shell×4 |

**Failures:**

**Run 1**: output does not contain 'folder3'

**Run 2**: output does not contain 'folder3'

**Run 3**: output does not contain 'folder3'

**Run 5**: output does not contain 'folder3'

**Run 6**: output does not contain 'folder3'

**Run 8**: output does not contain 'folder3'

**Run 9**: output does not contain 'folder3'

---

#### W1

**✓ Simple File Creation** | Create a new file with content | 100% (10/10) | 2.0 calls | 5954 tokens | 11.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 10.1s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 2 | ✓ PASS | 10.7s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 3 | ✓ PASS | 11.4s | 2 | 5950 | 49 | 3015 | $0.0000 | Write |
| 4 | ✓ PASS | 10.7s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 5 | ✓ PASS | 10.4s | 2 | 5951 | 50 | 3016 | $0.0000 | Write |
| 6 | ✓ PASS | 12.3s | 2 | 5959 | 58 | 3024 | $0.0000 | Write |
| 7 | ✓ PASS | 13.2s | 2 | 5963 | 62 | 3028 | $0.0000 | Write |
| 8 | ✓ PASS | 12.5s | 2 | 5960 | 59 | 3025 | $0.0000 | Write |
| 9 | ✓ PASS | 11.4s | 2 | 5950 | 49 | 3015 | $0.0000 | Write |
| 10 | ✓ PASS | 10.4s | 2 | 5950 | 49 | 3015 | $0.0000 | Write |

---

#### W2

**✓ Multi-line Content** | Write file with multiple lines and formatting | 100% (10/10) | 2.0 calls | 6011 tokens | 16.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 17.1s | 2 | 6019 | 87 | 3057 | $0.0000 | Write |
| 2 | ✓ PASS | 20.1s | 2 | 6029 | 97 | 3067 | $0.0000 | Write |
| 3 | ✓ PASS | 15.3s | 2 | 6005 | 73 | 3043 | $0.0000 | Write |
| 4 | ✓ PASS | 16.9s | 2 | 6005 | 81 | 3051 | $0.0000 | Write |
| 5 | ✓ PASS | 14.2s | 2 | 6000 | 68 | 3038 | $0.0000 | Write |
| 6 | ✓ PASS | 17.1s | 2 | 6015 | 83 | 3053 | $0.0000 | Write |
| 7 | ✓ PASS | 16.2s | 2 | 6009 | 77 | 3047 | $0.0000 | Write |
| 8 | ✓ PASS | 15.3s | 2 | 6005 | 73 | 3043 | $0.0000 | Write |
| 9 | ✓ PASS | 19.0s | 2 | 6022 | 90 | 3060 | $0.0000 | Write |
| 10 | ✓ PASS | 16.0s | 2 | 6001 | 77 | 3047 | $0.0000 | Write |

---

#### W3

**✗ Overwrite Existing** | Handle overwrite of existing file | 90% (9/10) | 2.9 calls | 8836 tokens | 15.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 13.3s | 3 | 9134 | 61 | 3140 | $0.0000 | Write, Write.confirm |
| 2 | ✓ PASS | 21.3s | 3 | 9205 | 97 | 3177 | $0.0000 | Write, Write.confirm |
| 3 | ✓ PASS | 14.3s | 3 | 9136 | 63 | 3142 | $0.0000 | Write, Write.confirm |
| 4 | ✗ FAIL | 13.9s | 2 | 6012 | 64 | 3079 | $0.0000 | Write |
| 5 | ✓ PASS | 14.2s | 3 | 9136 | 63 | 3142 | $0.0000 | Write, Write.confirm |
| 6 | ✓ PASS | 14.4s | 3 | 9136 | 63 | 3142 | $0.0000 | Write, Write.confirm |
| 7 | ✓ PASS | 17.3s | 3 | 9145 | 72 | 3151 | $0.0000 | Write, Write.confirm |
| 8 | ✓ PASS | 14.9s | 3 | 9139 | 66 | 3145 | $0.0000 | Write, Write.confirm |
| 9 | ✓ PASS | 16.0s | 3 | 9145 | 72 | 3151 | $0.0000 | Write, Write.confirm |
| 10 | ✓ PASS | 18.1s | 3 | 9177 | 83 | 3163 | $0.0000 | Write, Write.confirm |

**Failures:**

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


---

#### W4

**✓ Special Characters** | Handle special characters in content | 100% (10/10) | 2.0 calls | 5988 tokens | 12.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 16.7s | 2 | 6009 | 79 | 3056 | $0.0000 | Write |
| 2 | ✓ PASS | 11.3s | 2 | 5984 | 54 | 3031 | $0.0000 | Write |
| 3 | ✓ PASS | 13.9s | 2 | 5996 | 66 | 3043 | $0.0000 | Write |
| 4 | ✓ PASS | 11.4s | 2 | 5983 | 53 | 3030 | $0.0000 | Write |
| 5 | ✓ PASS | 11.1s | 2 | 5982 | 52 | 3029 | $0.0000 | Write |
| 6 | ✓ PASS | 13.7s | 2 | 5996 | 66 | 3043 | $0.0000 | Write |
| 7 | ✓ PASS | 11.2s | 2 | 5982 | 52 | 3029 | $0.0000 | Write |
| 8 | ✓ PASS | 11.1s | 2 | 5982 | 52 | 3029 | $0.0000 | Write |
| 9 | ✓ PASS | 11.3s | 2 | 5982 | 52 | 3029 | $0.0000 | Write |
| 10 | ✓ PASS | 11.2s | 2 | 5983 | 53 | 3030 | $0.0000 | Write |

---

#### W5

**✓ Empty File Creation** | Create an empty file | 100% (10/10) | 2.0 calls | 5930 tokens | 9.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 7.7s | 2 | 5924 | 37 | 2998 | $0.0000 | Write |
| 2 | ✓ PASS | 8.1s | 2 | 5924 | 37 | 2998 | $0.0000 | Write |
| 3 | ✓ PASS | 8.1s | 2 | 5924 | 37 | 2998 | $0.0000 | Write |
| 4 | ✓ PASS | 10.3s | 2 | 5935 | 48 | 3009 | $0.0000 | Write |
| 5 | ✓ PASS | 9.4s | 2 | 5931 | 44 | 3005 | $0.0000 | Write |
| 6 | ✓ PASS | 10.7s | 2 | 5937 | 50 | 3011 | $0.0000 | Write |
| 7 | ✓ PASS | 9.5s | 2 | 5931 | 44 | 3005 | $0.0000 | Write |
| 8 | ✓ PASS | 8.1s | 2 | 5924 | 37 | 2998 | $0.0000 | Write |
| 9 | ✓ PASS | 10.2s | 2 | 5936 | 49 | 3010 | $0.0000 | Write |
| 10 | ✓ PASS | 9.8s | 2 | 5933 | 46 | 3007 | $0.0000 | Write |

---

#### W6

**✓ Path with Spaces** | Handle paths with spaces | 100% (10/10) | 2.0 calls | 5967 tokens | 10.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 9.0s | 2 | 5960 | 44 | 3018 | $0.0000 | Write |
| 2 | ✓ PASS | 12.2s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 3 | ✓ PASS | 10.9s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 4 | ✓ PASS | 11.1s | 2 | 5968 | 52 | 3026 | $0.0000 | Write |
| 5 | ✓ PASS | 10.8s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 6 | ✓ PASS | 11.1s | 2 | 5968 | 52 | 3026 | $0.0000 | Write |
| 7 | ✓ PASS | 11.1s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 8 | ✓ PASS | 11.1s | 2 | 5968 | 52 | 3026 | $0.0000 | Write |
| 9 | ✓ PASS | 11.0s | 2 | 5967 | 51 | 3025 | $0.0000 | Write |
| 10 | ✓ PASS | 10.8s | 2 | 5968 | 52 | 3026 | $0.0000 | Write |

---

## Failure Analysis

| Benchmark | Run | Errors | Last Tool Call |
|-----------|-----|--------|----------------|
| S1 | 1 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 1 | output does not contain '7' | Shell |
| S4 | 1 | output does not contain 'folder3' | Read |
| E2 | 1 | file file.txt content does not match expected:
... | Edit |
| E3 | 1 | file data.txt content does not match expected:
... | Edit |
| E4 | 1 | file boundary.txt content does not match expect... | Edit |
| E5 | 1 | file code.ts content does not match expected:
-... | Edit |
| E6 | 1 | file values.ts content does not match expected:... | Edit |
| E7 | 1 | file func.ts content does not match expected:
-... | Edit |
| E8 | 1 | file sample.txt content does not match expected... | Edit |
| E9 | 1 | file numbered.txt content does not match expect... | Edit |
| E10 | 1 | file special.txt content does not match expecte... | Edit |
| E11 | 1 | file process.py content does not match expected... | Edit |
| C2 | 1 | file config.yaml content does not match expecte... | Edit |
| S2 | 2 | output does not contain '7' | Shell |
| S4 | 2 | output does not contain 'folder3' | Shell |
| E2 | 2 | file file.txt content does not match expected:
... | Edit |
| E3 | 2 | file data.txt content does not match expected:
... | Edit |
| E4 | 2 | file boundary.txt content does not match expect... | Edit |
| E5 | 2 | file code.ts content does not match expected:
-... | Edit |
| E6 | 2 | file values.ts content does not match expected:... | Edit |
| E7 | 2 | file func.ts content does not match expected:
-... | Edit |
| E8 | 2 | file sample.txt content does not match expected... | Edit |
| E10 | 2 | file special.txt content does not match expecte... | Edit |
| E11 | 2 | file process.py content does not match expected... | Edit |
| C3 | 2 | file utils.ts content does not match expected:
... | Edit |
| C5 | 2 | output does not contain 'wire.go' | Search |
| S2 | 3 | output does not contain '7' | Shell |
| S4 | 3 | output does not contain 'folder3' | Read |
| E3 | 3 | file data.txt content does not match expected:
... | Edit |
| E4 | 3 | file boundary.txt content does not match expect... | Edit |
| E5 | 3 | file code.ts content does not match expected:
-... | Edit |
| E6 | 3 | file values.ts content does not match expected:... | Edit |
| E7 | 3 | cancelled | Read |
| E8 | 3 | file sample.txt content does not match expected... | Edit |
| E10 | 3 | file special.txt content does not match expecte... | Edit |
| E11 | 3 | file process.py content does not match expected... | Edit |
| C1 | 3 | output does not contain 'db.example.com' | Read |
| S1 | 4 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 4 | output does not contain '7' | Shell |
| W3 | 4 | file existing.txt content does not match expect... | Write |
| E3 | 4 | file data.txt content does not match expected:
... | Edit |
| E4 | 4 | file boundary.txt content does not match expect... | Edit |
| E5 | 4 | file code.ts content does not match expected:
-... | Edit |
| E6 | 4 | file values.ts content does not match expected:... | Edit |
| E7 | 4 | file func.ts content does not match expected:
-... | Edit |
| E8 | 4 | file sample.txt content does not match expected... | Edit |
| E9 | 4 | file numbered.txt content does not match expect... | Edit |
| E10 | 4 | file special.txt content does not match expecte... | Edit |
| E11 | 4 | file process.py content does not match expected... | Edit |
| C2 | 4 | file config.yaml content does not match expecte... | Edit |
| C3 | 4 | file utils.ts content does not match expected:
... | Search |
| S2 | 5 | output does not contain '7' | Shell |
| S4 | 5 | output does not contain 'folder3' | Shell |
| E2 | 5 | file file.txt content does not match expected:
... | Edit |
| E3 | 5 | file data.txt content does not match expected:
... | Edit |
| E4 | 5 | file boundary.txt content does not match expect... | Edit |
| E5 | 5 | file code.ts content does not match expected:
-... | Edit |
| E6 | 5 | file values.ts content does not match expected:... | Edit |
| E7 | 5 | file func.ts content does not match expected:
-... | Edit |
| E8 | 5 | file sample.txt content does not match expected... | Edit |
| E10 | 5 | file special.txt content does not match expecte... | Edit |
| E11 | 5 | file process.py content does not match expected... | Edit |
| C2 | 5 | file config.yaml content does not match expecte... | Edit |
| C5 | 5 | output does not contain 'wire.go' | Search |
| S1 | 6 | output does not contain 'func calculateTotal(it... | Search |
| S2 | 6 | output does not contain '7' | Shell |
| S4 | 6 | output does not contain 'folder3' | Search |
| E2 | 6 | file file.txt content does not match expected:
... | Edit |
| E3 | 6 | file data.txt content does not match expected:
... | Edit |
| E4 | 6 | file boundary.txt content does not match expect... | Edit |
| E5 | 6 | file code.ts content does not match expected:
-... | Edit |
| E6 | 6 | file values.ts content does not match expected:... | Edit |
| E7 | 6 | file func.ts content does not match expected:
-... | Edit |
| E8 | 6 | file sample.txt content does not match expected... | Edit |
| E10 | 6 | file special.txt content does not match expecte... | Edit |
| E11 | 6 | file process.py content does not match expected... | Edit |
| C2 | 6 | file config.yaml content does not match expecte... | Edit |
| C3 | 6 | file utils.ts content does not match expected:
... | Search |
| S2 | 7 | output does not contain '7' | Shell |
| E2 | 7 | file file.txt content does not match expected:
... | Edit |
| E3 | 7 | file data.txt content does not match expected:
... | Edit |
| E4 | 7 | file boundary.txt content does not match expect... | Edit |
| E5 | 7 | file code.ts content does not match expected:
-... | Edit |
| E6 | 7 | file values.ts content does not match expected:... | Edit |
| E7 | 7 | file func.ts content does not match expected:
-... | Edit |
| E8 | 7 | file sample.txt content does not match expected... | Edit |
| E9 | 7 | file numbered.txt content does not match expect... | Edit |
| E10 | 7 | file special.txt content does not match expecte... | Edit |
| E11 | 7 | file process.py content does not match expected... | Edit |
| C2 | 7 | file config.yaml content does not match expecte... | Edit |
| C3 | 7 | file handler1.ts content does not match expecte... | Edit |
| S2 | 8 | output does not contain '7' | Shell |
| S4 | 8 | output does not contain 'folder3' | Read |
| E1 | 8 | file numbered.txt content does not match expect... | Edit |
| E2 | 8 | file file.txt content does not match expected:
... | Edit |
| E3 | 8 | file data.txt content does not match expected:
... | Edit |
| E4 | 8 | file boundary.txt content does not match expect... | Edit |
| E5 | 8 | file code.ts content does not match expected:
-... | Edit |
| E6 | 8 | file values.ts content does not match expected:... | Edit |
| E7 | 8 | file func.ts content does not match expected:
-... | Edit |
| E8 | 8 | file sample.txt content does not match expected... | Edit |
| E9 | 8 | file numbered.txt content does not match expect... | Edit |
| E10 | 8 | file special.txt content does not match expecte... | Edit |
| E11 | 8 | file process.py content does not match expected... | Edit |
| C3 | 8 | file handler1.ts content does not match expecte... | Read |
| S1 | 9 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 9 | output does not contain '7' | Shell |
| S4 | 9 | output does not contain 'folder3' | Shell |
| E2 | 9 | file file.txt content does not match expected:
... | Edit |
| E3 | 9 | file data.txt content does not match expected:
... | Edit |
| E4 | 9 | file boundary.txt content does not match expect... | Edit |
| E5 | 9 | file code.ts content does not match expected:
-... | Edit |
| E6 | 9 | file values.ts content does not match expected:... | Edit |
| E7 | 9 | file func.ts content does not match expected:
-... | Edit |
| E8 | 9 | file sample.txt content does not match expected... | Edit |
| E9 | 9 | file numbered.txt content does not match expect... | Edit |
| E10 | 9 | file special.txt content does not match expecte... | Edit |
| E11 | 9 | file process.py content does not match expected... | Edit |
| C2 | 9 | file config.yaml content does not match expecte... | Edit |
| C3 | 9 | file utils.ts content does not match expected:
... | Edit |
| S1 | 10 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 10 | output does not contain '7' | Shell |
| E2 | 10 | file file.txt content does not match expected:
... | Edit |
| E3 | 10 | file data.txt content does not match expected:
... | Edit |
| E4 | 10 | file boundary.txt content does not match expect... | Edit |
| E5 | 10 | file code.ts content does not match expected:
-... | Edit |
| E6 | 10 | file values.ts content does not match expected:... | Edit |
| E7 | 10 | file func.ts content does not match expected:
-... | Edit |
| E8 | 10 | file sample.txt content does not match expected... | Edit |
| E10 | 10 | file special.txt content does not match expecte... | Edit |
| E11 | 10 | file process.py content does not match expected... | Edit |
| C3 | 10 | file utils.ts content does not match expected:
... | Edit |

## Appendix A: Configuration

### Version

```
kvit-coder 9604c5e (commit 20260102, built 2026-01-03)
```

### config.yaml

```yaml

```

