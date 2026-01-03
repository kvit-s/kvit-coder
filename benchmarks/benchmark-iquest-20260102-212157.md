# LLM Tool Usage Benchmark Report

## Metadata

- **Version**: kvit-coder 68cbe77 (commit 20260102, built 2026-01-03)
- **Date**: 2026-01-02T22:13:56-06:00
- **Total Benchmarks**: 28
- **Total Runs**: 280

## Summary

| Class | Success Rate | Avg Time/Run |
|-------|--------------|--------------|
| C | 52% (21/40) | 98.8s |
| E | 23% (25/110) | 107.7s |
| R | 93% (28/30) | 20.0s |
| S | 42% (17/40) | 44.0s |
| W | 93% (56/60) | 32.2s |
| **Total** | **52% (147/280)** | **302.6s** |

## Detailed Statistics

### Per-Benchmark Summary

| Benchmark | Success | LLM Calls | Tokens | Generated | Context | Prompt Speed | Gen Speed | Cost | Duration |
|-----------|---------|-----------|--------|-----------|---------|--------------|-----------|------|----------|
| C1 | 10% | 2.2(±0.6) | 6612(±2092) | 71(±65) | 3057 | 240.9 t/s | 12.1 t/s | $0.0000 | 6.3s(±5.4) |
| C2 | 70% | 3.8(±1.4) | 11748(±4281) | 128(±56) | 2974 | 285.7 t/s | 10.0 t/s | $0.0000 | 26.7s(±31.4) |
| C3 | 50% | 7.4(±1.7) | 30152(±8310) | 462(±158) | 4988 | 421.9 t/s | 11.0 t/s | $0.0000 | 51.3s(±16.5) |
| C5 | 80% | 2.0 | 7578(±817) | 94(±35) | 4654 | 695.3 t/s | 16.3 t/s | $0.0000 | 14.4s(±5.1) |
| E1 | 100% | 2.0 | 6132(±9) | 78(±9) | 3180 | 215.5 t/s | 11.8 t/s | $0.0000 | 7.7s(±0.7) |
| E2 | 0% | 2.8(±0.4) | 9118(±1480) | 136(±37) | 3546 | 323.6 t/s | 11.3 t/s | $0.0000 | 13.6s(±3.5) |
| E3 | 0% | 3.0 | 9802(±104) | 130(±23) | 3594 | 333.1 t/s | 11.3 t/s | $0.0000 | 13.2s(±2.0) |
| E4 | 0% | 2.0 | 6094(±13) | 88(±8) | 3147 | 309.1 t/s | 12.0 t/s | $0.0000 | 7.9s(±0.6) |
| E5 | 40% | 2.9(±1.1) | 9802(±4553) | 151(±77) | 3639 | 490.2 t/s | 12.5 t/s | $0.0000 | 15.4s(±8.4) |
| E6 | 0% | 2.0 | 6011(±54) | 62(±8) | 3073 | 279.6 t/s | 11.8 t/s | $0.0000 | 5.8s(±0.8) |
| E7 | 0% | 2.0 | 6027(±57) | 67(±8) | 3085 | 278.2 t/s | 11.6 t/s | $0.0000 | 6.3s(±1.0) |
| E8 | 0% | 2.1(±0.3) | 6377(±986) | 74(±21) | 3128 | 317.7 t/s | 11.7 t/s | $0.0000 | 6.9s(±1.8) |
| E9 | 100% | 3.0 | 9554(±20) | 120(±20) | 3435 | 216.8 t/s | 11.9 t/s | $0.0000 | 12.2s(±1.6) |
| E10 | 0% | 2.0 | 6107(±12) | 93(±8) | 3154 | 302.9 t/s | 12.0 t/s | $0.0000 | 8.3s(±0.8) |
| E11 | 10% | 2.3(±0.5) | 7179(±1494) | 114(±21) | 3285 | 367.2 t/s | 11.6 t/s | $0.0000 | 10.5s(±2.1) |
| R1 | 90% | 2.0 | 6031(±5) | 67(±5) | 3119 | 495.0 t/s | 16.0 t/s | $0.0000 | 4.5s(±0.4) |
| R2 | 90% | 2.0 | 6710(±25) | 99(±25) | 3790 | 342.8 t/s | 11.8 t/s | $0.0000 | 10.7s(±2.3) |
| R3 | 100% | 2.0 | 6000(±7) | 52(±7) | 3082 | 316.9 t/s | 12.1 t/s | $0.0000 | 4.8s(±0.5) |
| S1 | 20% | 3.6(±0.7) | 11826(±2449) | 178(±61) | 3658 | 429.7 t/s | 14.1 t/s | $0.0000 | 16.1s(±5.9) |
| S2 | 90% | 2.0 | 6091(±96) | 138(±48) | 3073 | 484.6 t/s | 14.8 t/s | $0.0000 | 9.6s(±3.4) |
| S3 | 40% | 2.0 | 5986(±78) | 49(±13) | 3032 | 370.4 t/s | 15.5 t/s | $0.0000 | 3.5s(±0.9) |
| S4 | 20% | 2.8(±1.2) | 10589(±7324) | 218(±289) | 4067 | 2253.2 t/s | 23.2 t/s | $0.0000 | 14.8s(±17.7) |
| W1 | 100% | 2.0 | 5938(±5) | 54(±5) | 3012 | 204.4 t/s | 12.2 t/s | $0.0000 | 4.9s(±0.6) |
| W2 | 100% | 2.0 | 5993(±6) | 79(±6) | 3041 | 153.9 t/s | 11.6 t/s | $0.0000 | 7.5s(±0.5) |
| W3 | 70% | 2.7(±0.5) | 8174(±1432) | 60(±4) | 3113 | 262.6 t/s | 12.1 t/s | $0.0000 | 5.7s(±0.4) |
| W4 | 90% | 2.0 | 5966(±2) | 53(±1) | 3022 | 211.7 t/s | 11.9 t/s | $0.0000 | 4.9s(±0.2) |
| W5 | 100% | 2.0 | 5915(±5) | 45(±5) | 2998 | 193.0 t/s | 11.7 t/s | $0.0000 | 4.3s(±0.4) |
| W6 | 100% | 2.0 | 5950(±1) | 51(±1) | 3017 | 208.0 t/s | 11.7 t/s | $0.0000 | 4.8s(±0.1) |

### Per-Benchmark Details

#### C1

**✗ Search Then Read** | Find and read a file | 10% (1/10) | 2.2 calls | 6612 tokens | 6.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 3.8s | 2 | 5900 | 34 | 2979 | $0.0000 | Search |
| 2 | ✗ FAIL | 3.5s | 2 | 5903 | 37 | 2982 | $0.0000 | Search |
| 3 | ✗ FAIL | 4.7s | 2 | 5920 | 54 | 2999 | $0.0000 | Search |
| 4 | ✗ FAIL | 5.0s | 2 | 5925 | 59 | 3004 | $0.0000 | Search |
| 5 | ✗ FAIL | 5.2s | 2 | 5923 | 57 | 3002 | $0.0000 | Search |
| 6 | ✗ FAIL | 4.5s | 2 | 5914 | 48 | 2993 | $0.0000 | Search |
| 7 | ✓ PASS | 22.4s | 4 | 12887 | 263 | 3622 | $0.0000 | Read, Search×2 |
| 8 | ✗ FAIL | 4.5s | 2 | 5915 | 49 | 2994 | $0.0000 | Search |
| 9 | ✗ FAIL | 5.1s | 2 | 5924 | 58 | 3003 | $0.0000 | Search |
| 10 | ✗ FAIL | 4.5s | 2 | 5913 | 47 | 2992 | $0.0000 | Search |

**Failures:**

**Run 1**: output does not contain 'db.example.com'

**Run 2**: output does not contain 'db.example.com'

**Run 3**: output does not contain 'db.example.com'

**Run 4**: output does not contain 'db.example.com'

**Run 5**: output does not contain 'db.example.com'

**Run 6**: output does not contain 'db.example.com'

**Run 8**: output does not contain 'db.example.com'

**Run 9**: output does not contain 'db.example.com'

**Run 10**: output does not contain 'db.example.com'

---

#### C2

**✗ Read-Modify-Write** | Complete edit workflow | 70% (7/10) | 3.8 calls | 11748 tokens | 26.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 15.3s | 5 | 15278 | 158 | 3229 | $0.0000 | Shell, Write, Write.confirm |
| 2 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 3 | ✗ FAIL | 21.5s | 3 | 9424 | 115 | 3362 | $0.0000 | Edit, Read |
| 4 | ✓ PASS | 13.6s | 4 | 12738 | 124 | 3467 | $0.0000 | Edit, Read, Shell |
| 5 | ✓ PASS | 16.4s | 5 | 15275 | 155 | 3243 | $0.0000 | Shell, Write, Write.confirm |
| 6 | ✓ PASS | 18.7s | 5 | 15274 | 154 | 3238 | $0.0000 | Shell, Write, Write.confirm |
| 7 | ✗ FAIL | 25.3s | 4 | 12487 | 237 | 3475 | $0.0000 | Edit, Read |
| 8 | ✓ PASS | 12.0s | 4 | 12330 | 107 | 3238 | $0.0000 | Shell, Write, Write.confirm |
| 9 | ✓ PASS | 12.0s | 4 | 12330 | 107 | 3238 | $0.0000 | Shell, Write, Write.confirm |
| 10 | ✓ PASS | 12.4s | 4 | 12341 | 118 | 3249 | $0.0000 | Shell, Write, Write.confirm |

**Failures:**

**Run 2**: 
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


**Run 3**: 
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


**Run 7**: 
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


---

#### C3

**✗ Search-Read-Edit** | Find, understand, and modify | 50% (5/10) | 7.4 calls | 30152 tokens | 51.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 56.8s | 8 | 32767 | 519 | 5147 | $0.0000 | Edit×3, Read×3, Search |
| 2 | ✗ FAIL | 58.6s | 9 | 38460 | 521 | 5547 | $0.0000 | Edit×3, Read×3, Search×2 |
| 3 | ✗ FAIL | 17.7s | 4 | 14045 | 146 | 3939 | $0.0000 | Edit, Read, Search |
| 4 | ✓ PASS | 53.5s | 8 | 32878 | 470 | 5144 | $0.0000 | Edit×3, Read×3, Search |
| 5 | ✗ FAIL | 58.3s | 8 | 33009 | 540 | 5269 | $0.0000 | Edit×3, Read×3, Search |
| 6 | ✓ PASS | 60.0s | 8 | 32762 | 533 | 5167 | $0.0000 | Edit×3, Read×3, Search |
| 7 | ✓ PASS | 68.7s | 9 | 37656 | 639 | 5294 | $0.0000 | Edit×4, Read×3, Search |
| 8 | ✓ PASS | 54.5s | 8 | 32901 | 490 | 5161 | $0.0000 | Edit×3, Read×3, Search |
| 9 | ✗ FAIL | 21.2s | 4 | 14006 | 174 | 3922 | $0.0000 | Read×2, Search |
| 10 | ✗ FAIL | 64.0s | 8 | 33037 | 591 | 5295 | $0.0000 | Edit×3, Read×3, Search |

**Failures:**

**Run 2**: 
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


**Run 3**: 
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


**Run 5**: 
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
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -3,5 +3,5 @@
 }
 
 function deprecatedFunc() {}
-function newFunc() {}
+        function newFunc() {}
 

```


---

#### C5

**✗ Needle in Haystack Search** | Find specific initialization among many usages | 80% (8/10) | 2.0 calls | 7578 tokens | 14.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 12.7s | 2 | 7946 | 63 | 5026 | $0.0000 | Search |
| 2 | ✓ PASS | 15.0s | 2 | 7978 | 95 | 5058 | $0.0000 | Search |
| 3 | ✓ PASS | 15.5s | 2 | 7978 | 95 | 5058 | $0.0000 | Search |
| 4 | ✓ PASS | 14.6s | 2 | 7973 | 90 | 5053 | $0.0000 | Search |
| 5 | ✓ PASS | 14.7s | 2 | 7966 | 83 | 5046 | $0.0000 | Search |
| 6 | ✓ PASS | 16.6s | 2 | 7988 | 105 | 5068 | $0.0000 | Search |
| 7 | ✗ FAIL | 5.5s | 2 | 5934 | 55 | 3005 | $0.0000 | Search |
| 8 | ✓ PASS | 16.8s | 2 | 7987 | 104 | 5067 | $0.0000 | Search |
| 9 | ✗ FAIL | 7.3s | 2 | 5956 | 63 | 3015 | $0.0000 | Search |
| 10 | ✓ PASS | 25.3s | 2 | 8069 | 186 | 5149 | $0.0000 | Search |

**Failures:**

**Run 7**: output does not contain 'wire.go'

**Run 9**: output does not contain 'wire.go'

---

#### E1

**✓ Single Line Replace** | Replace a single line | 100% (10/10) | 2.0 calls | 6132 tokens | 7.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 7.5s | 2 | 6127 | 72 | 3174 | $0.0000 | Edit |
| 2 | ✓ PASS | 8.6s | 2 | 6146 | 93 | 3195 | $0.0000 | Edit |
| 3 | ✓ PASS | 7.2s | 2 | 6128 | 73 | 3175 | $0.0000 | Edit |
| 4 | ✓ PASS | 9.4s | 2 | 6151 | 96 | 3198 | $0.0000 | Edit |
| 5 | ✓ PASS | 7.3s | 2 | 6128 | 73 | 3175 | $0.0000 | Edit |
| 6 | ✓ PASS | 7.9s | 2 | 6134 | 79 | 3181 | $0.0000 | Edit |
| 7 | ✓ PASS | 7.2s | 2 | 6128 | 73 | 3175 | $0.0000 | Edit |
| 8 | ✓ PASS | 7.0s | 2 | 6124 | 71 | 3173 | $0.0000 | Edit |
| 9 | ✓ PASS | 7.3s | 2 | 6128 | 73 | 3175 | $0.0000 | Edit |
| 10 | ✓ PASS | 7.2s | 2 | 6127 | 72 | 3174 | $0.0000 | Edit |

---

#### E2

**✗ Multi-line Insert** | Insert multiple lines at position | 0% (0/10) | 2.8 calls | 9118 tokens | 13.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 11.3s | 3 | 9757 | 113 | 3549 | $0.0000 | Edit, Read |
| 2 | ✗ FAIL | 13.4s | 3 | 9830 | 130 | 3602 | $0.0000 | Edit, Read |
| 3 | ✗ FAIL | 15.7s | 3 | 9859 | 150 | 3628 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 14.6s | 3 | 9849 | 142 | 3620 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 13.5s | 3 | 9834 | 139 | 3614 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 7.5s | 2 | 6116 | 75 | 3161 | $0.0000 | Edit |
| 7 | ✗ FAIL | 14.5s | 3 | 9834 | 136 | 3608 | $0.0000 | Edit, Read |
| 8 | ✗ FAIL | 9.5s | 2 | 6204 | 104 | 3248 | $0.0000 | Edit |
| 9 | ✗ FAIL | 15.6s | 3 | 9955 | 146 | 3709 | $0.0000 | Edit, Read |
| 10 | ✗ FAIL | 20.7s | 3 | 9941 | 225 | 3722 | $0.0000 | Edit, Read |

**Failures:**

**Run 1**: 
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


**Run 5**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,10 +7,10 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
+        INSERTED
+        INSERTED
+        INSERTED
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
@@ -1,4 +1,6 @@
-FUNCTION A
+INSERTED
+INSERTED
+INSERTED
 FUNCTION B
 FUNCTION C
 FUNCTION D
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


**Run 7**: 
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


**Run 8**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,11 +8,11 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
 FUNCTION K
 FUNCTION L
+INSERTED
+    INSERTED
+    INSERTED
 FUNCTION M
 FUNCTION N
 FUNCTION O

```


**Run 9**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -7,11 +7,11 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
-INSERTED
-INSERTED
-INSERTED
-FUNCTION K
+   10│FUNCTION J
+   11│INSERTED
+   12│INSERTED
+   13│INSERTED
+   14│FUNCTION K
 FUNCTION L
 FUNCTION M
 FUNCTION N

```


**Run 10**: 
```diff
file file.txt content does not match expected:
--- expected
+++ actual
@@ -8,9 +8,12 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
+
+
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

**✗ Delete Lines** | Delete a range of lines | 0% (0/10) | 3.0 calls | 9802 tokens | 13.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 12.8s | 3 | 9730 | 124 | 3535 | $0.0000 | Edit, Read |
| 2 | ✗ FAIL | 16.5s | 3 | 9978 | 162 | 3730 | $0.0000 | Edit, Read |
| 3 | ✗ FAIL | 14.6s | 3 | 9933 | 149 | 3711 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 16.1s | 3 | 9905 | 170 | 3683 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 14.8s | 3 | 9892 | 143 | 3673 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 11.3s | 3 | 9710 | 104 | 3515 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 11.5s | 3 | 9715 | 109 | 3520 | $0.0000 | Edit, Read |
| 8 | ✗ FAIL | 12.5s | 3 | 9729 | 123 | 3534 | $0.0000 | Edit, Read |
| 9 | ✗ FAIL | 10.8s | 3 | 9712 | 106 | 3517 | $0.0000 | Edit, Read |
| 10 | ✗ FAIL | 11.3s | 3 | 9715 | 109 | 3520 | $0.0000 | Edit, Read |

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
@@ -8,6 +8,8 @@
 function H
 function I
 function J
+function L
+function M
 function O
 function P
 function Q

```


**Run 3**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,16 @@
 function H
 function I
 function J
+function L
+function M
+function N
+function O
+function P
+function Q
+function R
+function S
+function T
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
@@ -8,10 +8,14 @@
 function H
 function I
 function J
+function L
+function M
+function N
 function O
 function P
 function Q
 function R
 function S
 function T
+function T
 

```


**Run 5**: 
```diff
file data.txt content does not match expected:
--- expected
+++ actual
@@ -8,6 +8,14 @@
 function H
 function I
 function J
+function L
+function M
+function O
+function P
+function Q
+function R
+function S
+function T
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

**✗ Boundary Edit** | Test boundary conditions | 0% (0/10) | 2.0 calls | 6094 tokens | 7.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 7.2s | 2 | 6074 | 77 | 3127 | $0.0000 | Edit |
| 2 | ✗ FAIL | 7.9s | 2 | 6095 | 85 | 3148 | $0.0000 | Edit |
| 3 | ✗ FAIL | 7.6s | 2 | 6082 | 85 | 3135 | $0.0000 | Edit |
| 4 | ✗ FAIL | 8.9s | 2 | 6099 | 102 | 3152 | $0.0000 | Edit |
| 5 | ✗ FAIL | 8.0s | 2 | 6111 | 89 | 3164 | $0.0000 | Edit |
| 6 | ✗ FAIL | 7.9s | 2 | 6099 | 89 | 3152 | $0.0000 | Edit |
| 7 | ✗ FAIL | 6.9s | 2 | 6074 | 77 | 3127 | $0.0000 | Edit |
| 8 | ✗ FAIL | 8.3s | 2 | 6114 | 92 | 3167 | $0.0000 | Edit |
| 9 | ✗ FAIL | 8.5s | 2 | 6097 | 100 | 3150 | $0.0000 | Edit |
| 10 | ✗ FAIL | 7.9s | 2 | 6097 | 87 | 3150 | $0.0000 | Edit |

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
@@ -1,5 +1,5 @@
 function A
-function B
+cleanup done
 function C
 function D
 function E
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
@@ -1,6 +1,6 @@
 function A
 function B
-function C
+cleanup done
 function D
 function E
 function F
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
@@ -1,5 +1,5 @@
 function A
-function B
+cleanup done
 function C
 function D
 function E
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
@@ -1,6 +1,6 @@
 function A
 function B
-function C
+cleanup done
 function D
 function E
 function F
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
@@ -1,5 +1,5 @@
 function A
-function B
+cleanup done
 function C
 function D
 function E
@@ -17,5 +17,5 @@
 function Q
 function R
 function S
-cleanup done
+function T
 

```


---

#### E5

**✗ Replace All Occurrences** | Replace multiple occurrences | 40% (4/10) | 2.9 calls | 9802 tokens | 15.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 21.7s | 4 | 14528 | 200 | 4277 | $0.0000 | Edit, Read, Search |
| 2 | ✗ FAIL | 8.8s | 2 | 6130 | 91 | 3184 | $0.0000 | Edit |
| 3 | ✗ FAIL | 10.2s | 2 | 6148 | 109 | 3202 | $0.0000 | Edit |
| 4 | ✓ PASS | 26.3s | 5 | 17472 | 246 | 4284 | $0.0000 | Edit, Read, Search |
| 5 | ✗ FAIL | 8.3s | 2 | 6128 | 89 | 3182 | $0.0000 | Edit |
| 6 | ✓ PASS | 22.7s | 4 | 14535 | 207 | 4280 | $0.0000 | Edit, Read, Search |
| 7 | ✗ FAIL | 8.8s | 2 | 6132 | 93 | 3186 | $0.0000 | Edit |
| 8 | ✗ FAIL | 8.4s | 2 | 6125 | 86 | 3179 | $0.0000 | Edit |
| 9 | ✓ PASS | 30.4s | 4 | 14634 | 306 | 4379 | $0.0000 | Edit, Read, Search |
| 10 | ✗ FAIL | 8.2s | 2 | 6192 | 87 | 3234 | $0.0000 | Edit |

**Failures:**

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


**Run 7**: 
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


**Run 10**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,12 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+  console.log('new');
+}
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

**✗ Context-Specific Replace** | Replace with context for uniqueness | 0% (0/10) | 2.0 calls | 6011 tokens | 5.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 6.0s | 2 | 6042 | 63 | 3097 | $0.0000 | Edit |
| 2 | ✗ FAIL | 6.7s | 2 | 6053 | 74 | 3108 | $0.0000 | Edit |
| 3 | ✗ FAIL | 4.3s | 2 | 5924 | 45 | 3006 | $0.0000 | Read |
| 4 | ✗ FAIL | 4.9s | 2 | 5933 | 54 | 3015 | $0.0000 | Read |
| 5 | ✗ FAIL | 6.2s | 2 | 6047 | 68 | 3102 | $0.0000 | Edit |
| 6 | ✗ FAIL | 6.4s | 2 | 6047 | 68 | 3102 | $0.0000 | Edit |
| 7 | ✗ FAIL | 5.6s | 2 | 6043 | 64 | 3098 | $0.0000 | Edit |
| 8 | ✗ FAIL | 4.9s | 2 | 5930 | 56 | 3007 | $0.0000 | Search |
| 9 | ✗ FAIL | 6.2s | 2 | 6042 | 63 | 3097 | $0.0000 | Edit |
| 10 | ✗ FAIL | 6.6s | 2 | 6047 | 68 | 3102 | $0.0000 | Edit |

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

**✗ Multi-line Block Replace** | Replace multi-line block | 0% (0/10) | 2.0 calls | 6027 tokens | 6.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 7.2s | 2 | 6066 | 73 | 3115 | $0.0000 | Edit |
| 2 | ✗ FAIL | 6.9s | 2 | 6064 | 71 | 3113 | $0.0000 | Edit |
| 3 | ✗ FAIL | 7.0s | 2 | 6064 | 71 | 3113 | $0.0000 | Edit |
| 4 | ✗ FAIL | 5.0s | 2 | 5940 | 55 | 3019 | $0.0000 | Read |
| 5 | ✗ FAIL | 5.0s | 2 | 5945 | 60 | 3024 | $0.0000 | Read |
| 6 | ✗ FAIL | 7.0s | 2 | 6067 | 74 | 3116 | $0.0000 | Edit |
| 7 | ✗ FAIL | 6.7s | 2 | 6065 | 72 | 3114 | $0.0000 | Edit |
| 8 | ✗ FAIL | 6.5s | 2 | 6064 | 71 | 3113 | $0.0000 | Edit |
| 9 | ✗ FAIL | 6.8s | 2 | 6064 | 71 | 3113 | $0.0000 | Edit |
| 10 | ✗ FAIL | 4.5s | 2 | 5934 | 49 | 3013 | $0.0000 | Read |

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

**✗ No Match Handling** | Handle search text not found | 0% (0/10) | 2.1 calls | 6377 tokens | 6.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 6.8s | 2 | 6054 | 73 | 3119 | $0.0000 | Edit |
| 2 | ✗ FAIL | 6.6s | 2 | 6054 | 74 | 3111 | $0.0000 | Edit |
| 3 | ✗ FAIL | 6.5s | 2 | 6054 | 73 | 3119 | $0.0000 | Edit |
| 4 | ✗ FAIL | 6.7s | 2 | 6046 | 66 | 3103 | $0.0000 | Edit |
| 5 | ✗ FAIL | 6.0s | 2 | 6044 | 63 | 3109 | $0.0000 | Edit |
| 6 | ✗ FAIL | 6.6s | 2 | 6051 | 71 | 3108 | $0.0000 | Edit |
| 7 | ✗ FAIL | 6.2s | 2 | 6049 | 68 | 3114 | $0.0000 | Edit |
| 8 | ✗ FAIL | 4.8s | 2 | 6028 | 47 | 3093 | $0.0000 | Edit |
| 9 | ✗ FAIL | 6.7s | 2 | 6056 | 76 | 3113 | $0.0000 | Edit |
| 10 | ✗ FAIL | 12.2s | 3 | 9336 | 132 | 3286 | $0.0000 | Edit, Read |

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

output contains 'replaced' but should not
output contains 'successfully' but should not

**Run 2**: 
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

output contains 'replaced' but should not
output contains 'successfully' but should not

**Run 4**: 
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

output contains 'replaced' but should not
output contains 'successfully' but should not

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

**Run 6**: 
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

output contains 'replaced' but should not
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
@@ -1,4 +1,4 @@
-This is a sample file.
+replacement
 It has multiple lines.
 Nothing special here.
 

```

output contains 'successfully' but should not

**Run 10**: output contains 'replaced' but should not

---

#### E9

**✓ Empty Content** | Handle empty replacement | 100% (10/10) | 3.0 calls | 9554 tokens | 12.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 11.5s | 3 | 9545 | 111 | 3426 | $0.0000 | Edit, Read |
| 2 | ✓ PASS | 12.5s | 3 | 9558 | 124 | 3439 | $0.0000 | Edit, Read |
| 3 | ✓ PASS | 14.1s | 3 | 9578 | 144 | 3459 | $0.0000 | Edit, Read |
| 4 | ✓ PASS | 11.1s | 3 | 9538 | 104 | 3419 | $0.0000 | Edit, Read |
| 5 | ✓ PASS | 12.5s | 3 | 9556 | 122 | 3437 | $0.0000 | Edit, Read |
| 6 | ✓ PASS | 9.5s | 3 | 9523 | 89 | 3404 | $0.0000 | Edit, Read |
| 7 | ✓ PASS | 11.4s | 3 | 9541 | 107 | 3422 | $0.0000 | Edit, Read |
| 8 | ✓ PASS | 15.8s | 3 | 9599 | 165 | 3480 | $0.0000 | Edit, Read |
| 9 | ✓ PASS | 11.5s | 3 | 9548 | 114 | 3429 | $0.0000 | Edit, Read |
| 10 | ✓ PASS | 11.9s | 3 | 9556 | 122 | 3437 | $0.0000 | Edit, Read |

---

#### E10

**✗ Special Characters** | Handle special characters in replacement | 0% (0/10) | 2.0 calls | 6107 tokens | 8.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 9.8s | 2 | 6114 | 104 | 3163 | $0.0000 | Edit |
| 2 | ✗ FAIL | 7.3s | 2 | 6092 | 82 | 3141 | $0.0000 | Edit |
| 3 | ✗ FAIL | 8.1s | 2 | 6118 | 94 | 3159 | $0.0000 | Edit |
| 4 | ✗ FAIL | 9.3s | 2 | 6128 | 103 | 3168 | $0.0000 | Edit |
| 5 | ✗ FAIL | 8.3s | 2 | 6118 | 94 | 3159 | $0.0000 | Edit |
| 6 | ✗ FAIL | 7.5s | 2 | 6094 | 84 | 3143 | $0.0000 | Edit |
| 7 | ✗ FAIL | 7.6s | 2 | 6095 | 85 | 3144 | $0.0000 | Edit |
| 8 | ✗ FAIL | 8.3s | 2 | 6105 | 95 | 3154 | $0.0000 | Edit |
| 9 | ✗ FAIL | 7.4s | 2 | 6094 | 84 | 3143 | $0.0000 | Edit |
| 10 | ✗ FAIL | 9.0s | 2 | 6114 | 104 | 3163 | $0.0000 | Edit |

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
@@ -1,6 +1,6 @@
-line A
+value = $100 + 50%
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
-line A
+value = $100 + 50%
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
-line A
+value = $100 + 50%
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

**✗ Indentation Preservation** | Maintain correct indentation (critical for Python) | 10% (1/10) | 2.3 calls | 7179 tokens | 10.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 9.6s | 2 | 6202 | 102 | 3235 | $0.0000 | Edit |
| 2 | ✗ FAIL | 9.9s | 2 | 6210 | 104 | 3251 | $0.0000 | Edit |
| 3 | ✗ FAIL | 15.5s | 3 | 9177 | 164 | 3241 | $0.0000 | Edit |
| 4 | ✗ FAIL | 9.3s | 2 | 6202 | 101 | 3235 | $0.0000 | Edit |
| 5 | ✓ PASS | 12.8s | 3 | 9618 | 134 | 3488 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 11.9s | 3 | 9569 | 127 | 3453 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 8.1s | 2 | 6188 | 86 | 3221 | $0.0000 | Edit |
| 8 | ✗ FAIL | 9.4s | 2 | 6208 | 104 | 3241 | $0.0000 | Edit |
| 9 | ✗ FAIL | 9.1s | 2 | 6205 | 104 | 3238 | $0.0000 | Edit |
| 10 | ✗ FAIL | 9.7s | 2 | 6210 | 110 | 3243 | $0.0000 | Edit |

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
+        # placeholder
         finalize()
-
+validate_input()
+transform_data()

```


**Run 2**: 
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
+transform_data()
 

```


**Run 4**: 
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
+transform_data()
+        # placeholder
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: expected an indented block after 'if' statement on line 2 (process.py, line 3)

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
+validate_input()
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

**Run 8**: 
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

**Run 9**: 
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
+transform_data()
+        # placeholder
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: expected an indented block after 'if' statement on line 2 (process.py, line 3)

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
+        # placeholder
         finalize()
-
+validate_input()
+transform_data()

```


---

#### R1

**✗ Simple File Read** | Read an entire small file | 90% (9/10) | 2.0 calls | 6031 tokens | 4.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.0s | 2 | 6024 | 60 | 3112 | $0.0000 | Read |
| 2 | ✓ PASS | 4.5s | 2 | 6032 | 68 | 3120 | $0.0000 | Read |
| 3 | ✓ PASS | 4.7s | 2 | 6035 | 71 | 3123 | $0.0000 | Read |
| 4 | ✓ PASS | 4.5s | 2 | 6033 | 69 | 3121 | $0.0000 | Read |
| 5 | ✓ PASS | 3.9s | 2 | 6024 | 60 | 3112 | $0.0000 | Read |
| 6 | ✓ PASS | 4.5s | 2 | 6035 | 71 | 3123 | $0.0000 | Read |
| 7 | ✓ PASS | 4.5s | 2 | 6035 | 71 | 3123 | $0.0000 | Read |
| 8 | ✗ FAIL | 4.3s | 2 | 6031 | 67 | 3119 | $0.0000 | Read |
| 9 | ✓ PASS | 4.9s | 2 | 6035 | 71 | 3123 | $0.0000 | Read |
| 10 | ✓ PASS | 5.4s | 2 | 6022 | 58 | 3110 | $0.0000 | Read |

**Failures:**

**Run 8**: output does not contain 'db_host=localhost'

---

#### R2

**✗ Truncation Recovery** | Handle truncated output by chunked reading | 90% (9/10) | 2.0 calls | 6710 tokens | 10.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 12.5s | 2 | 6736 | 125 | 3816 | $0.0000 | Read |
| 2 | ✓ PASS | 13.4s | 2 | 6742 | 131 | 3822 | $0.0000 | Read |
| 3 | ✓ PASS | 9.4s | 2 | 6699 | 88 | 3779 | $0.0000 | Read |
| 4 | ✓ PASS | 9.8s | 2 | 6704 | 93 | 3784 | $0.0000 | Read |
| 5 | ✓ PASS | 10.1s | 2 | 6704 | 93 | 3784 | $0.0000 | Read |
| 6 | ✓ PASS | 14.0s | 2 | 6743 | 132 | 3823 | $0.0000 | Read |
| 7 | ✓ PASS | 12.7s | 2 | 6727 | 116 | 3807 | $0.0000 | Read |
| 8 | ✓ PASS | 9.9s | 2 | 6699 | 88 | 3779 | $0.0000 | Read |
| 9 | ✓ PASS | 8.6s | 2 | 6688 | 77 | 3768 | $0.0000 | Read |
| 10 | ✗ FAIL | 6.3s | 2 | 6660 | 49 | 3740 | $0.0000 | Read |

**Failures:**

**Run 10**: output does not contain 'SECTION_ALPHA'
output does not contain 'SECTION_BETA'
output does not contain 'SECTION_GAMMA'
output does not contain 'SECTION_DELTA'
output does not contain 'SECTION_EPSILON'

---

#### R3

**✓ Relative vs Absolute Path** | Correct path resolution | 100% (10/10) | 2.0 calls | 6000 tokens | 4.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.6s | 2 | 5995 | 48 | 3077 | $0.0000 | Read |
| 2 | ✓ PASS | 4.6s | 2 | 5998 | 51 | 3080 | $0.0000 | Read |
| 3 | ✓ PASS | 4.2s | 2 | 5994 | 46 | 3076 | $0.0000 | Read |
| 4 | ✓ PASS | 4.4s | 2 | 5994 | 46 | 3076 | $0.0000 | Read |
| 5 | ✓ PASS | 4.1s | 2 | 5991 | 44 | 3073 | $0.0000 | Read |
| 6 | ✓ PASS | 6.0s | 2 | 6013 | 66 | 3095 | $0.0000 | Read |
| 7 | ✓ PASS | 4.9s | 2 | 6005 | 58 | 3087 | $0.0000 | Read |
| 8 | ✓ PASS | 5.0s | 2 | 6000 | 52 | 3082 | $0.0000 | Read |
| 9 | ✓ PASS | 5.0s | 2 | 6000 | 53 | 3082 | $0.0000 | Read |
| 10 | ✓ PASS | 5.3s | 2 | 6007 | 60 | 3089 | $0.0000 | Read |

---

#### S1

**✗ Simple Pattern Search** | Find a specific function definition | 20% (2/10) | 3.6 calls | 11826 tokens | 16.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 20.0s | 4 | 13230 | 183 | 3748 | $0.0000 | Read×2, Search |
| 2 | ✗ FAIL | 22.3s | 4 | 13337 | 194 | 3795 | $0.0000 | Read×2, Search |
| 3 | ✗ FAIL | 4.1s | 2 | 5919 | 46 | 2999 | $0.0000 | Search |
| 4 | ✗ FAIL | 11.9s | 3 | 9608 | 120 | 3514 | $0.0000 | Read, Search |
| 5 | ✗ FAIL | 19.8s | 4 | 13291 | 228 | 3798 | $0.0000 | Read×2, Search |
| 6 | ✗ FAIL | 15.7s | 4 | 13220 | 183 | 3741 | $0.0000 | Read×2, Search |
| 7 | ✗ FAIL | 11.0s | 3 | 9616 | 128 | 3522 | $0.0000 | Read, Search |
| 8 | ✓ PASS | 25.3s | 4 | 13408 | 265 | 3866 | $0.0000 | Read×2, Search |
| 9 | ✓ PASS | 16.8s | 4 | 13302 | 239 | 3810 | $0.0000 | Read×2, Search |
| 10 | ✗ FAIL | 13.9s | 4 | 13333 | 190 | 3791 | $0.0000 | Read×2, Search |

**Failures:**

**Run 1**: output does not contain 'func calculateTotal(items []int)'

**Run 2**: output does not contain 'func calculateTotal(items []int)'

**Run 3**: output does not contain 'func calculateTotal(items []int)'

**Run 4**: output does not contain 'func calculateTotal(items []int)'

**Run 5**: output does not contain 'func calculateTotal(items []int)'

**Run 6**: output does not contain 'func calculateTotal(items []int)'

**Run 7**: output does not contain 'func calculateTotal(items []int)'

**Run 10**: output does not contain 'func calculateTotal(items []int)'

---

#### S2

**✗ Multi-Pattern Search** | Find multiple related items | 90% (9/10) | 2.0 calls | 6091 tokens | 9.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 8.8s | 2 | 6066 | 125 | 3060 | $0.0000 | Shell |
| 2 | ✓ PASS | 13.8s | 2 | 6212 | 197 | 3132 | $0.0000 | Shell |
| 3 | ✓ PASS | 4.9s | 2 | 5958 | 71 | 3006 | $0.0000 | Shell |
| 4 | ✓ PASS | 13.3s | 2 | 6204 | 194 | 3129 | $0.0000 | Shell |
| 5 | ✓ PASS | 14.7s | 2 | 6236 | 209 | 3144 | $0.0000 | Shell |
| 6 | ✓ PASS | 8.7s | 2 | 6066 | 125 | 3060 | $0.0000 | Shell |
| 7 | ✓ PASS | 8.1s | 2 | 6054 | 119 | 3054 | $0.0000 | Shell |
| 8 | ✓ PASS | 8.0s | 2 | 6052 | 117 | 3052 | $0.0000 | Shell |
| 9 | ✓ PASS | 10.9s | 2 | 6113 | 160 | 3095 | $0.0000 | Shell |
| 10 | ✗ FAIL | 4.5s | 2 | 5946 | 65 | 3000 | $0.0000 | Shell |

**Failures:**

**Run 10**: output does not contain '7'

---

#### S3

**✗ Search with File Filtering** | Search in specific file types | 40% (4/10) | 2.0 calls | 5986 tokens | 3.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.7s | 2 | 5990 | 66 | 3018 | $0.0000 | Shell |
| 2 | ✗ FAIL | 2.6s | 2 | 5942 | 37 | 2999 | $0.0000 | Search |
| 3 | ✓ PASS | 4.0s | 2 | 6211 | 53 | 3252 | $0.0000 | Search |
| 4 | ✗ FAIL | 2.7s | 2 | 5942 | 37 | 2999 | $0.0000 | Search |
| 5 | ✗ FAIL | 2.6s | 2 | 5942 | 37 | 2999 | $0.0000 | Search |
| 6 | ✗ FAIL | 2.7s | 2 | 5942 | 37 | 2999 | $0.0000 | Search |
| 7 | ✗ FAIL | 3.0s | 2 | 5951 | 46 | 3008 | $0.0000 | Search |
| 8 | ✓ PASS | 5.2s | 2 | 6004 | 73 | 3025 | $0.0000 | Shell |
| 9 | ✓ PASS | 4.4s | 2 | 5982 | 61 | 3015 | $0.0000 | Shell |
| 10 | ✗ FAIL | 3.2s | 2 | 5952 | 47 | 3009 | $0.0000 | Search |

**Failures:**

**Run 2**: output does not contain 'models.go'

**Run 4**: output does not contain 'models.go'

**Run 5**: output does not contain 'models.go'

**Run 6**: output does not contain 'models.go'

**Run 7**: output does not contain 'models.go'

**Run 10**: output does not contain 'models.go'

---

#### S4

**✗ Search Truncation Recovery** | Handle large search results requiring refinement | 20% (2/10) | 2.8 calls | 10589 tokens | 14.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 4.4s | 2 | 5932 | 67 | 3004 | $0.0000 | Shell |
| 2 | ✓ PASS | 66.7s | 6 | 30792 | 1068 | 5837 | $0.0000 | Search, Shell×3 |
| 3 | ✗ FAIL | 5.2s | 2 | 7790 | 43 | 4876 | $0.0000 | Search |
| 4 | ✗ FAIL | 4.5s | 2 | 5936 | 71 | 3008 | $0.0000 | Shell |
| 5 | ✗ FAIL | 17.6s | 4 | 13768 | 231 | 4891 | $0.0000 | Search |
| 6 | ✗ FAIL | 13.7s | 3 | 12795 | 196 | 5003 | $0.0000 | Search |
| 7 | ✓ PASS | 6.7s | 2 | 6002 | 96 | 3031 | $0.0000 | Shell |
| 8 | ✗ FAIL | 10.3s | 3 | 10759 | 119 | 4867 | $0.0000 | Search |
| 9 | ✗ FAIL | 10.8s | 2 | 6090 | 162 | 3095 | $0.0000 | Shell |
| 10 | ✗ FAIL | 8.4s | 2 | 6025 | 122 | 3055 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain 'folder3'

**Run 3**: output does not contain 'folder3'

**Run 4**: output does not contain 'folder3'

**Run 5**: output does not contain 'folder3'

**Run 6**: output does not contain 'folder3'

**Run 8**: output does not contain 'folder3'

**Run 9**: output does not contain 'folder3'

**Run 10**: output does not contain 'folder3'

---

#### W1

**✓ Simple File Creation** | Create a new file with content | 100% (10/10) | 2.0 calls | 5938 tokens | 4.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.7s | 2 | 5934 | 50 | 3008 | $0.0000 | Write |
| 2 | ✓ PASS | 4.2s | 2 | 5933 | 49 | 3007 | $0.0000 | Write |
| 3 | ✓ PASS | 4.7s | 2 | 5934 | 50 | 3008 | $0.0000 | Write |
| 4 | ✓ PASS | 4.5s | 2 | 5934 | 50 | 3008 | $0.0000 | Write |
| 5 | ✓ PASS | 5.3s | 2 | 5945 | 61 | 3019 | $0.0000 | Write |
| 6 | ✓ PASS | 4.2s | 2 | 5933 | 49 | 3007 | $0.0000 | Write |
| 7 | ✓ PASS | 5.7s | 2 | 5945 | 61 | 3019 | $0.0000 | Write |
| 8 | ✓ PASS | 4.5s | 2 | 5934 | 50 | 3008 | $0.0000 | Write |
| 9 | ✓ PASS | 6.0s | 2 | 5943 | 59 | 3017 | $0.0000 | Write |
| 10 | ✓ PASS | 5.6s | 2 | 5945 | 61 | 3019 | $0.0000 | Write |

---

#### W2

**✓ Multi-line Content** | Write file with multiple lines and formatting | 100% (10/10) | 2.0 calls | 5993 tokens | 7.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 7.1s | 2 | 5988 | 73 | 3035 | $0.0000 | Write |
| 2 | ✓ PASS | 7.9s | 2 | 5998 | 83 | 3045 | $0.0000 | Write |
| 3 | ✓ PASS | 8.1s | 2 | 6003 | 88 | 3050 | $0.0000 | Write |
| 4 | ✓ PASS | 7.5s | 2 | 5993 | 78 | 3040 | $0.0000 | Write |
| 5 | ✓ PASS | 6.9s | 2 | 5987 | 80 | 3042 | $0.0000 | Write |
| 6 | ✓ PASS | 7.0s | 2 | 5988 | 73 | 3035 | $0.0000 | Write |
| 7 | ✓ PASS | 8.3s | 2 | 6002 | 87 | 3049 | $0.0000 | Write |
| 8 | ✓ PASS | 7.7s | 2 | 5998 | 83 | 3045 | $0.0000 | Write |
| 9 | ✓ PASS | 7.3s | 2 | 5988 | 73 | 3035 | $0.0000 | Write |
| 10 | ✓ PASS | 7.2s | 2 | 5988 | 73 | 3035 | $0.0000 | Write |

---

#### W3

**✗ Overwrite Existing** | Handle overwrite of existing file | 70% (7/10) | 2.7 calls | 8174 tokens | 5.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 5.7s | 2 | 5993 | 62 | 3069 | $0.0000 | Write |
| 2 | ✓ PASS | 5.9s | 3 | 9110 | 61 | 3133 | $0.0000 | Write, Write.confirm |
| 3 | ✓ PASS | 6.2s | 3 | 9112 | 63 | 3135 | $0.0000 | Write, Write.confirm |
| 4 | ✓ PASS | 5.8s | 3 | 9110 | 61 | 3133 | $0.0000 | Write, Write.confirm |
| 5 | ✓ PASS | 6.0s | 3 | 9112 | 63 | 3135 | $0.0000 | Write, Write.confirm |
| 6 | ✓ PASS | 5.9s | 3 | 9110 | 61 | 3133 | $0.0000 | Write, Write.confirm |
| 7 | ✓ PASS | 6.0s | 3 | 9112 | 63 | 3135 | $0.0000 | Write, Write.confirm |
| 8 | ✓ PASS | 6.0s | 3 | 9112 | 63 | 3135 | $0.0000 | Write, Write.confirm |
| 9 | ✗ FAIL | 4.8s | 2 | 5985 | 54 | 3061 | $0.0000 | Write |
| 10 | ✗ FAIL | 5.0s | 2 | 5983 | 52 | 3059 | $0.0000 | Write |

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

**✗ Special Characters** | Handle special characters in content | 90% (9/10) | 2.0 calls | 5966 tokens | 4.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 5.0s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 2 | ✓ PASS | 5.0s | 2 | 5965 | 52 | 3021 | $0.0000 | Write |
| 3 | ✓ PASS | 4.8s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 4 | ✓ PASS | 5.0s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 5 | ✗ FAIL | 5.2s | 2 | 5971 | 56 | 3024 | $0.0000 | Write |
| 6 | ✓ PASS | 4.8s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 7 | ✓ PASS | 4.8s | 2 | 5965 | 52 | 3021 | $0.0000 | Write |
| 8 | ✓ PASS | 4.7s | 2 | 5965 | 52 | 3021 | $0.0000 | Write |
| 9 | ✓ PASS | 5.2s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 10 | ✓ PASS | 4.5s | 2 | 5963 | 50 | 3019 | $0.0000 | Write |

**Failures:**

**Run 5**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1,4 @@
-'quotes', "double", `backticks`, $var
-
+quotes
+"double"
+`backticks`
+$var

```


---

#### W5

**✓ Empty File Creation** | Create an empty file | 100% (10/10) | 2.0 calls | 5915 tokens | 4.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.1s | 2 | 5913 | 43 | 2996 | $0.0000 | Write |
| 2 | ✓ PASS | 4.2s | 2 | 5919 | 49 | 3002 | $0.0000 | Write |
| 3 | ✓ PASS | 4.3s | 2 | 5918 | 48 | 3001 | $0.0000 | Write |
| 4 | ✓ PASS | 4.6s | 2 | 5919 | 49 | 3002 | $0.0000 | Write |
| 5 | ✓ PASS | 3.6s | 2 | 5907 | 37 | 2990 | $0.0000 | Write |
| 6 | ✓ PASS | 4.6s | 2 | 5919 | 49 | 3002 | $0.0000 | Write |
| 7 | ✓ PASS | 5.1s | 2 | 5919 | 49 | 3002 | $0.0000 | Write |
| 8 | ✓ PASS | 4.4s | 2 | 5916 | 46 | 2999 | $0.0000 | Write |
| 9 | ✓ PASS | 4.6s | 2 | 5916 | 46 | 2999 | $0.0000 | Write |
| 10 | ✓ PASS | 3.8s | 2 | 5907 | 37 | 2990 | $0.0000 | Write |

---

#### W6

**✓ Path with Spaces** | Handle paths with spaces | 100% (10/10) | 2.0 calls | 5950 tokens | 4.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 5.0s | 2 | 5951 | 52 | 3018 | $0.0000 | Write |
| 2 | ✓ PASS | 4.9s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 3 | ✓ PASS | 4.7s | 2 | 5951 | 52 | 3018 | $0.0000 | Write |
| 4 | ✓ PASS | 5.0s | 2 | 5949 | 50 | 3016 | $0.0000 | Write |
| 5 | ✓ PASS | 4.7s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 6 | ✓ PASS | 4.8s | 2 | 5949 | 50 | 3016 | $0.0000 | Write |
| 7 | ✓ PASS | 4.6s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 8 | ✓ PASS | 4.7s | 2 | 5949 | 50 | 3016 | $0.0000 | Write |
| 9 | ✓ PASS | 4.7s | 2 | 5951 | 52 | 3018 | $0.0000 | Write |
| 10 | ✓ PASS | 4.7s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |

---

## Failure Analysis

| Benchmark | Run | Errors | Last Tool Call |
|-----------|-----|--------|----------------|
| S1 | 1 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 2 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 3 | output does not contain 'func calculateTotal(it... | Search |
| S1 | 4 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 5 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 6 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 7 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 10 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 10 | output does not contain '7' | Shell |
| S3 | 2 | output does not contain 'models.go' | Search |
| S3 | 4 | output does not contain 'models.go' | Search |
| S3 | 5 | output does not contain 'models.go' | Search |
| S3 | 6 | output does not contain 'models.go' | Search |
| S3 | 7 | output does not contain 'models.go' | Search |
| S3 | 10 | output does not contain 'models.go' | Search |
| S4 | 1 | output does not contain 'folder3' | Shell |
| S4 | 3 | output does not contain 'folder3' | Search |
| S4 | 4 | output does not contain 'folder3' | Shell |
| S4 | 5 | output does not contain 'folder3' | Search |
| S4 | 6 | output does not contain 'folder3' | Search |
| S4 | 8 | output does not contain 'folder3' | Search |
| S4 | 9 | output does not contain 'folder3' | Shell |
| S4 | 10 | output does not contain 'folder3' | Shell |
| R1 | 8 | output does not contain 'db_host=localhost' | Read |
| R2 | 10 | output does not contain 'SECTION_ALPHA'; output... | Read |
| W3 | 1 | file existing.txt content does not match expect... | Write |
| W3 | 9 | file existing.txt content does not match expect... | Write |
| W3 | 10 | file existing.txt content does not match expect... | Write |
| W4 | 5 | file special.txt content does not match expecte... | Write |
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
| E5 | 2 | file code.ts content does not match expected:
-... | Edit |
| E5 | 3 | file code.ts content does not match expected:
-... | Edit |
| E5 | 5 | file code.ts content does not match expected:
-... | Edit |
| E5 | 7 | file code.ts content does not match expected:
-... | Edit |
| E5 | 8 | file code.ts content does not match expected:
-... | Edit |
| E5 | 10 | file code.ts content does not match expected:
-... | Edit |
| E6 | 1 | file values.ts content does not match expected:... | Edit |
| E6 | 2 | file values.ts content does not match expected:... | Edit |
| E6 | 3 | file values.ts content does not match expected:... | Read |
| E6 | 4 | file values.ts content does not match expected:... | Read |
| E6 | 5 | file values.ts content does not match expected:... | Edit |
| E6 | 6 | file values.ts content does not match expected:... | Edit |
| E6 | 7 | file values.ts content does not match expected:... | Edit |
| E6 | 8 | file values.ts content does not match expected:... | Search |
| E6 | 9 | file values.ts content does not match expected:... | Edit |
| E6 | 10 | file values.ts content does not match expected:... | Edit |
| E7 | 1 | file func.ts content does not match expected:
-... | Edit |
| E7 | 2 | file func.ts content does not match expected:
-... | Edit |
| E7 | 3 | file func.ts content does not match expected:
-... | Edit |
| E7 | 4 | file func.ts content does not match expected:
-... | Read |
| E7 | 5 | file func.ts content does not match expected:
-... | Read |
| E7 | 6 | file func.ts content does not match expected:
-... | Edit |
| E7 | 7 | file func.ts content does not match expected:
-... | Edit |
| E7 | 8 | file func.ts content does not match expected:
-... | Edit |
| E7 | 9 | file func.ts content does not match expected:
-... | Edit |
| E7 | 10 | file func.ts content does not match expected:
-... | Read |
| E8 | 1 | file sample.txt content does not match expected... | Edit |
| E8 | 2 | file sample.txt content does not match expected... | Edit |
| E8 | 3 | file sample.txt content does not match expected... | Edit |
| E8 | 4 | file sample.txt content does not match expected... | Edit |
| E8 | 5 | file sample.txt content does not match expected... | Edit |
| E8 | 6 | file sample.txt content does not match expected... | Edit |
| E8 | 7 | file sample.txt content does not match expected... | Edit |
| E8 | 8 | file sample.txt content does not match expected... | Edit |
| E8 | 9 | file sample.txt content does not match expected... | Edit |
| E8 | 10 | output contains 'replaced' but should not | Edit |
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
| E11 | 3 | file process.py content does not match expected... | Edit |
| E11 | 4 | file process.py content does not match expected... | Edit |
| E11 | 6 | file process.py content does not match expected... | Edit |
| E11 | 7 | file process.py content does not match expected... | Edit |
| E11 | 8 | file process.py content does not match expected... | Edit |
| E11 | 9 | file process.py content does not match expected... | Edit |
| E11 | 10 | file process.py content does not match expected... | Edit |
| C1 | 1 | output does not contain 'db.example.com' | Search |
| C1 | 2 | output does not contain 'db.example.com' | Search |
| C1 | 3 | output does not contain 'db.example.com' | Search |
| C1 | 4 | output does not contain 'db.example.com' | Search |
| C1 | 5 | output does not contain 'db.example.com' | Search |
| C1 | 6 | output does not contain 'db.example.com' | Search |
| C1 | 8 | output does not contain 'db.example.com' | Search |
| C1 | 9 | output does not contain 'db.example.com' | Search |
| C1 | 10 | output does not contain 'db.example.com' | Search |
| C2 | 2 | file config.yaml content does not match expecte... |  |
| C2 | 3 | file config.yaml content does not match expecte... | Edit |
| C2 | 7 | file config.yaml content does not match expecte... | Edit |
| C3 | 2 | file utils.ts content does not match expected:
... | Search |
| C3 | 3 | file handler2.ts content does not match expecte... | Edit |
| C3 | 5 | file utils.ts content does not match expected:
... | Edit |
| C3 | 9 | file handler1.ts content does not match expecte... | Read |
| C3 | 10 | file utils.ts content does not match expected:
... | Edit |
| C5 | 7 | output does not contain 'wire.go' | Search |
| C5 | 9 | output does not contain 'wire.go' | Search |

## Appendix A: Configuration

### Version

```
kvit-coder 68cbe77 (commit 20260102, built 2026-01-03)
```

### config.yaml

```yaml

```

