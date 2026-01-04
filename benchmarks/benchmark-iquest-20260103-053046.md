# LLM Tool Usage Benchmark Report

## Metadata

- **Version**: kvit-coder baaf1fc (commit 20260103, built 2026-01-03)
- **Date**: 2026-01-03T06:13:35-06:00
- **Total Benchmarks**: 28
- **Total Runs**: 280

## Summary

| Class | Success Rate | Avg Time/Run |
|-------|--------------|--------------|
| C | 42% (17/40) | 78.0s |
| E | 28% (31/110) | 109.9s |
| R | 90% (27/30) | 13.8s |
| S | 32% (13/40) | 29.7s |
| W | 97% (58/60) | 22.6s |
| **Total** | **52% (146/280)** | **254.1s** |

## Detailed Statistics

### Per-Benchmark Summary

| Benchmark | Success | LLM Calls | Tokens | Generated | Context | Prompt Speed | Gen Speed | Cost | Duration |
|-----------|---------|-----------|--------|-----------|---------|--------------|-----------|------|----------|
| C1 | 10% | 2.7(±1.4) | 9873(±5657) | 95(±99) | 3712 | 984.6 t/s | 23.9 t/s | $0.0000 | 4.4s(±3.1) |
| C2 | 60% | 3.5(±1.9) | 12976(±6851) | 104(±59) | 3150 | 592.5 t/s | 16.1 t/s | $0.0000 | 31.3s(±44.7) |
| C3 | 80% | 11.0 | 56590(±89) | 587(±37) | 6720 | 1111.9 t/s | 21.2 t/s | $0.0000 | 37.2s(±32.8) |
| C5 | 20% | 2.1(±0.3) | 8444(±2382) | 78(±14) | 4568 | 1520.2 t/s | 18.9 t/s | $0.0000 | 5.1s(±3.0) |
| E1 | 100% | 3.0 | 11397(±3) | 90(±1) | 4046 | 759.2 t/s | 16.3 t/s | $0.0000 | 6.5s(±5.3) |
| E2 | 10% | 4.3(±0.6) | 17481(±3057) | 174(±41) | 4580 | 1067.9 t/s | 17.7 t/s | $0.0000 | 12.7s(±12.2) |
| E3 | 100% | 4.7(±1.6) | 19269(±7501) | 172(±88) | 4565 | 774.8 t/s | 18.3 t/s | $0.0000 | 11.2s(±7.9) |
| E4 | 0% | 3.0 | 11253(±14) | 99(±4) | 3958 | 918.1 t/s | 16.3 t/s | $0.0000 | 6.8s(±5.2) |
| E5 | 40% | 5.0(±1.9) | 21443(±8873) | 270(±157) | 4959 | 1523.1 t/s | 22.4 t/s | $0.0000 | 15.6s(±9.3) |
| E6 | 20% | 3.5(±1.3) | 13311(±5483) | 117(±79) | 3998 | 841.1 t/s | 12.7 t/s | $0.0000 | 12.0s(±16.8) |
| E7 | 0% | 3.0 | 11223(±6) | 91(±5) | 3915 | 551.5 t/s | 13.7 t/s | $0.0000 | 7.7s(±6.4) |
| E8 | 20% | 3.4(±0.5) | 12739(±2006) | 99(±26) | 3943 | 514.2 t/s | 13.9 t/s | $0.0000 | 8.3s(±7.6) |
| E9 | 20% | 3.4(±0.7) | 13079(±2789) | 107(±21) | 4126 | 649.3 t/s | 14.4 t/s | $0.0000 | 8.8s(±7.0) |
| E10 | 0% | 3.0 | 11312(±16) | 110(±10) | 3988 | 699.8 t/s | 14.3 t/s | $0.0000 | 8.7s(±7.0) |
| E11 | 0% | 4.0(±0.9) | 15722(±3736) | 140(±40) | 4338 | 972.5 t/s | 16.3 t/s | $0.0000 | 11.4s(±9.5) |
| R1 | 100% | 2.0 | 7195(±5) | 65(±5) | 3700 | 727.9 t/s | 17.9 t/s | $0.0000 | 4.4s(±3.3) |
| R2 | 70% | 2.0 | 7850(±23) | 73(±23) | 4347 | 1147.2 t/s | 22.0 t/s | $0.0000 | 6.4s(±7.0) |
| R3 | 100% | 2.0 | 7162(±4) | 48(±5) | 3661 | 718.6 t/s | 18.6 t/s | $0.0000 | 3.0s(±2.3) |
| S1 | 10% | 3.4(±0.9) | 13101(±3931) | 174(±89) | 4158 | 835.0 t/s | 19.4 t/s | $0.0000 | 10.1s(±8.7) |
| S2 | 90% | 2.0 | 7207(±114) | 112(±57) | 3630 | 1765.2 t/s | 25.8 t/s | $0.0000 | 8.7s(±11.6) |
| S3 | 0% | 2.0 | 7113(±5) | 41(±4) | 3586 | 634.8 t/s | 17.9 t/s | $0.0000 | 2.6s(±1.7) |
| S4 | 30% | 3.3(±1.7) | 15630(±12695) | 175(±138) | 5364 | 1510.8 t/s | 25.2 t/s | $0.0000 | 8.4s(±4.4) |
| W1 | 100% | 2.0 | 7107(±6) | 57(±6) | 3598 | 575.8 t/s | 18.0 t/s | $0.0000 | 3.5s(±2.6) |
| W2 | 100% | 2.0 | 7163(±8) | 82(±8) | 3627 | 537.7 t/s | 17.2 t/s | $0.0000 | 5.1s(±4.1) |
| W3 | 90% | 2.9(±0.3) | 10488(±1116) | 60(±5) | 3708 | 613.8 t/s | 16.4 t/s | $0.0000 | 4.3s(±3.4) |
| W4 | 90% | 2.0 | 7136(±10) | 57(±10) | 3609 | 588.3 t/s | 17.5 t/s | $0.0000 | 3.6s(±2.6) |
| W5 | 100% | 2.0 | 7078(±5) | 42(±5) | 3578 | 530.8 t/s | 17.9 t/s | $0.0000 | 2.7s(±2.1) |
| W6 | 100% | 2.0 | 7120(±8) | 55(±8) | 3604 | 740.7 t/s | 18.2 t/s | $0.0000 | 3.3s(±2.4) |

### Per-Benchmark Details

#### C1

**✗ Search Then Read** | Find and read a file | 10% (1/10) | 2.7 calls | 9873 tokens | 4.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.4s | 2 | 7081 | 49 | 3577 | $0.0000 | Search |
| 2 | ✗ FAIL | 1.6s | 2 | 7064 | 32 | 3560 | $0.0000 | Search |
| 3 | ✗ FAIL | 8.7s | 2 | 7089 | 57 | 3585 | $0.0000 | Search |
| 4 | ✗ FAIL | 8.8s | 2 | 7091 | 59 | 3587 | $0.0000 | Search |
| 5 | ✗ FAIL | 3.6s | 2 | 7085 | 53 | 3581 | $0.0000 | Search |
| 6 | ✓ PASS | 7.5s | 5 | 19120 | 271 | 4267 | $0.0000 | Read, Search×3 |
| 7 | ✗ FAIL | 1.6s | 2 | 7085 | 53 | 3581 | $0.0000 | Search |
| 8 | ✗ FAIL | 7.4s | 6 | 22987 | 310 | 4264 | $0.0000 | Read×2, Search×3 |
| 9 | ✗ FAIL | 1.2s | 2 | 7064 | 32 | 3560 | $0.0000 | Search |
| 10 | ✗ FAIL | 1.2s | 2 | 7064 | 32 | 3560 | $0.0000 | Search |

**Failures:**

**Run 1**: output does not contain 'db.example.com'

**Run 2**: output does not contain 'db.example.com'

**Run 3**: output does not contain 'db.example.com'

**Run 4**: output does not contain 'db.example.com'

**Run 5**: output does not contain 'db.example.com'

**Run 7**: output does not contain 'db.example.com'

**Run 8**: output does not contain 'db.example.com'

**Run 9**: output does not contain 'db.example.com'

**Run 10**: output does not contain 'db.example.com'

---

#### C2

**✗ Read-Modify-Write** | Complete edit workflow | 60% (6/10) | 3.5 calls | 12976 tokens | 31.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 12.4s | 6 | 21718 | 203 | 3821 | $0.0000 | Shell, Write, Write.confirm |
| 2 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 3 | ✓ PASS | 16.8s | 4 | 14653 | 98 | 3812 | $0.0000 | Shell, Write, Write.confirm |
| 4 | ✗ FAIL | 22.6s | 4 | 15303 | 127 | 4142 | $0.0000 | Edit, Edit.confirm, Read |
| 5 | ✓ PASS | 4.9s | 4 | 14668 | 112 | 3825 | $0.0000 | Shell, Write, Write.confirm |
| 6 | ✓ PASS | 3.9s | 4 | 14665 | 110 | 3824 | $0.0000 | Shell, Write, Write.confirm |
| 7 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 8 | ✗ FAIL | 5.5s | 5 | 18816 | 160 | 4134 | $0.0000 | Edit, Edit.confirm, Read |
| 9 | ✓ PASS | 3.6s | 4 | 14665 | 109 | 3822 | $0.0000 | Shell, Write, Write.confirm |
| 10 | ✓ PASS | 3.9s | 4 | 15276 | 116 | 4123 | $0.0000 | Edit, Edit.confirm, Read |

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


**Run 4**: 
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


---

#### C3

**✗ Search-Read-Edit** | Find, understand, and modify | 80% (8/10) | 11.0 calls | 56590 tokens | 37.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 37.6s | 11 | 56570 | 630 | 6723 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 2 | ✓ PASS | 29.8s | 11 | 56567 | 610 | 6699 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 3 | ✓ PASS | 106.4s | 11 | 56538 | 584 | 6673 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 4 | ✓ PASS | 96.1s | 11 | 56587 | 521 | 6659 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 5 | ✓ PASS | 17.7s | 11 | 56677 | 613 | 6755 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 6 | ✗ FAIL | 16.0s | 11 | 56459 | 528 | 6688 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 7 | ✓ PASS | 18.3s | 11 | 56590 | 624 | 6798 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 8 | ✓ PASS | 17.9s | 11 | 56571 | 615 | 6787 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 9 | ✗ FAIL | 17.2s | 11 | 56807 | 581 | 6760 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |
| 10 | ✓ PASS | 15.1s | 11 | 56531 | 567 | 6659 | $0.0000 | Edit×3, Edit.confirm×3, Read×3, Se... |

**Failures:**

**Run 6**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -2,6 +2,5 @@
     newFunc();
 }
 
-function deprecatedFunc() {}
 function newFunc() {}
 

```


**Run 9**: 
```diff
file handler1.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,5 @@
 function process() {
     newFunc();
+    deprecatedFunc();
 }
 

```


```diff
file handler2.ts content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,5 @@
 function handle() {
     newFunc();
+    deprecatedFunc();
 }
 

```


---

#### C5

**✗ Needle in Haystack Search** | Find specific initialization among many usages | 20% (2/10) | 2.1 calls | 8444 tokens | 5.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 5.1s | 2 | 7146 | 84 | 3619 | $0.0000 | Search |
| 2 | ✓ PASS | 6.0s | 2 | 9151 | 102 | 5648 | $0.0000 | Search |
| 3 | ✗ FAIL | 10.2s | 2 | 7104 | 59 | 3592 | $0.0000 | Search |
| 4 | ✗ FAIL | 10.3s | 2 | 7113 | 68 | 3603 | $0.0000 | Search |
| 5 | ✗ FAIL | 4.0s | 3 | 10786 | 94 | 3682 | $0.0000 | Search×2 |
| 6 | ✗ FAIL | 2.0s | 2 | 7114 | 58 | 3592 | $0.0000 | Search |
| 7 | ✗ FAIL | 2.7s | 2 | 7126 | 90 | 3599 | $0.0000 | Search |
| 8 | ✗ FAIL | 2.2s | 2 | 7126 | 70 | 3604 | $0.0000 | Search |
| 9 | ✓ PASS | 5.9s | 2 | 14650 | 83 | 11136 | $0.0000 | Search |
| 10 | ✗ FAIL | 2.2s | 2 | 7122 | 70 | 3605 | $0.0000 | Search |

**Failures:**

**Run 1**: output does not contain 'wire.go'

**Run 3**: output does not contain 'wire.go'

**Run 4**: output does not contain 'wire.go'

**Run 5**: output does not contain 'wire.go'

**Run 6**: output does not contain 'wire.go'

**Run 7**: output does not contain 'wire.go'

**Run 8**: output does not contain 'wire.go'

**Run 10**: output does not contain 'wire.go'

---

#### E1

**✓ Single Line Replace** | Replace a single line | 100% (10/10) | 3.0 calls | 11397 tokens | 6.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 5.2s | 3 | 11398 | 90 | 4045 | $0.0000 | Edit, Edit.confirm |
| 2 | ✓ PASS | 6.4s | 3 | 11399 | 91 | 4046 | $0.0000 | Edit, Edit.confirm |
| 3 | ✓ PASS | 4.0s | 3 | 11393 | 89 | 4044 | $0.0000 | Edit, Edit.confirm |
| 4 | ✓ PASS | 17.6s | 3 | 11399 | 91 | 4046 | $0.0000 | Edit, Edit.confirm |
| 5 | ✓ PASS | 16.3s | 3 | 11391 | 87 | 4044 | $0.0000 | Edit, Edit.confirm |
| 6 | ✓ PASS | 3.1s | 3 | 11401 | 93 | 4048 | $0.0000 | Edit, Edit.confirm |
| 7 | ✓ PASS | 3.2s | 3 | 11398 | 90 | 4045 | $0.0000 | Edit, Edit.confirm |
| 8 | ✓ PASS | 3.2s | 3 | 11399 | 91 | 4046 | $0.0000 | Edit, Edit.confirm |
| 9 | ✓ PASS | 3.2s | 3 | 11399 | 91 | 4046 | $0.0000 | Edit, Edit.confirm |
| 10 | ✓ PASS | 3.2s | 3 | 11395 | 91 | 4046 | $0.0000 | Edit, Edit.confirm |

---

#### E2

**✗ Multi-line Insert** | Insert multiple lines at position | 10% (1/10) | 4.3 calls | 17481 tokens | 12.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 9.5s | 4 | 16019 | 148 | 4464 | $0.0000 | Edit, Edit.confirm, Read |
| 2 | ✗ FAIL | 8.9s | 4 | 16055 | 141 | 4485 | $0.0000 | Edit, Edit.confirm, Read |
| 3 | ✓ PASS | 6.9s | 4 | 15988 | 146 | 4454 | $0.0000 | Edit, Edit.confirm, Read |
| 4 | ✗ FAIL | 27.6s | 4 | 16143 | 152 | 4528 | $0.0000 | Edit, Edit.confirm, Read |
| 5 | ✗ FAIL | 43.9s | 5 | 19699 | 250 | 4540 | $0.0000 | Edit, Edit.confirm, Read |
| 6 | ✗ FAIL | 5.7s | 4 | 16216 | 177 | 4577 | $0.0000 | Edit, Edit.confirm, Read |
| 7 | ✗ FAIL | 5.2s | 4 | 16193 | 154 | 4554 | $0.0000 | Edit, Edit.confirm, Read |
| 8 | ✗ FAIL | 4.7s | 4 | 16003 | 133 | 4452 | $0.0000 | Edit, Edit.confirm, Read |
| 9 | ✗ FAIL | 6.5s | 4 | 16423 | 192 | 4689 | $0.0000 | Edit, Edit.confirm, Read |
| 10 | ✗ FAIL | 8.4s | 6 | 26070 | 251 | 5054 | $0.0000 | Edit×2, Edit.cancel, Edit.confirm, Read |

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
@@ -8,9 +8,9 @@
 FUNCTION H
 FUNCTION I
 FUNCTION J
-INSERTED
-INSERTED
-INSERTED
+    INSERTED
+    INSERTED
+    INSERTED
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
+      INSERTED
+      INSERTED
+      INSERTED
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
+      INSERTED
+      INSERTED
+      INSERTED
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
+        INSERTED
+        INSERTED
+        INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 8**: 
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
@@ -7,7 +7,6 @@
 FUNCTION G
 FUNCTION H
 FUNCTION I
-FUNCTION J
 INSERTED
 INSERTED
 INSERTED

```


---

#### E3

**✓ Delete Lines** | Delete a range of lines | 100% (10/10) | 4.7 calls | 19269 tokens | 11.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 8.2s | 4 | 15909 | 137 | 4403 | $0.0000 | Edit, Edit.confirm, Read |
| 2 | ✓ PASS | 16.3s | 9 | 40130 | 271 | 5497 | $0.0000 | Edit×3, Edit.cancel×2, Edit.confirm... |
| 3 | ✓ PASS | 16.2s | 6 | 25336 | 404 | 4966 | $0.0000 | Edit×2, Edit.cancel, Edit.confirm, Read |
| 4 | ✓ PASS | 24.0s | 4 | 15900 | 128 | 4394 | $0.0000 | Edit, Edit.confirm, Read |
| 5 | ✓ PASS | 24.6s | 4 | 15902 | 130 | 4396 | $0.0000 | Edit, Edit.confirm, Read |
| 6 | ✓ PASS | 4.3s | 4 | 15906 | 134 | 4400 | $0.0000 | Edit, Edit.confirm, Read |
| 7 | ✓ PASS | 5.3s | 4 | 15891 | 120 | 4389 | $0.0000 | Edit, Edit.confirm, Read |
| 8 | ✓ PASS | 4.3s | 4 | 15906 | 134 | 4400 | $0.0000 | Edit, Edit.confirm, Read |
| 9 | ✓ PASS | 4.5s | 4 | 15901 | 130 | 4399 | $0.0000 | Edit, Edit.confirm, Read |
| 10 | ✓ PASS | 4.6s | 4 | 15908 | 137 | 4406 | $0.0000 | Edit, Edit.confirm, Read |

---

#### E4

**✗ Boundary Edit** | Test boundary conditions | 0% (0/10) | 3.0 calls | 11253 tokens | 6.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 6.0s | 3 | 11248 | 102 | 3956 | $0.0000 | Edit, Edit.confirm |
| 2 | ✗ FAIL | 7.9s | 3 | 11248 | 102 | 3956 | $0.0000 | Edit, Edit.confirm |
| 3 | ✗ FAIL | 4.3s | 3 | 11279 | 94 | 3974 | $0.0000 | Edit, Edit.confirm |
| 4 | ✗ FAIL | 17.8s | 3 | 11247 | 101 | 3955 | $0.0000 | Edit, Edit.confirm |
| 5 | ✗ FAIL | 15.9s | 3 | 11240 | 94 | 3948 | $0.0000 | Edit, Edit.confirm |
| 6 | ✗ FAIL | 3.3s | 3 | 11248 | 102 | 3956 | $0.0000 | Edit, Edit.confirm |
| 7 | ✗ FAIL | 3.3s | 3 | 11248 | 102 | 3956 | $0.0000 | Edit, Edit.confirm |
| 8 | ✗ FAIL | 3.4s | 3 | 11250 | 104 | 3958 | $0.0000 | Edit, Edit.confirm |
| 9 | ✗ FAIL | 3.2s | 3 | 11240 | 94 | 3948 | $0.0000 | Edit, Edit.confirm |
| 10 | ✗ FAIL | 3.1s | 3 | 11279 | 94 | 3974 | $0.0000 | Edit, Edit.confirm |

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

**✗ Replace All Occurrences** | Replace multiple occurrences | 40% (4/10) | 5.0 calls | 21443 tokens | 15.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 18.2s | 7 | 29088 | 308 | 5243 | $0.0000 | Edit×2, Edit.cancel, Edit.confirm, Read |
| 2 | ✓ PASS | 16.6s | 5 | 22715 | 264 | 5554 | $0.0000 | Edit, Edit.confirm, Read, Search |
| 3 | ✗ FAIL | 39.3s | 6 | 26260 | 663 | 5457 | $0.0000 | Edit×2, Edit.cancel, Edit.confirm, Read |
| 4 | ✗ FAIL | 18.4s | 2 | 7441 | 109 | 3901 | $0.0000 | Edit |
| 5 | ✗ FAIL | 19.7s | 3 | 11587 | 107 | 4165 | $0.0000 | Edit, Edit.confirm |
| 6 | ✓ PASS | 12.6s | 7 | 31339 | 390 | 5475 | $0.0000 | Edit, Edit.confirm, Read, Search×2 |
| 7 | ✓ PASS | 8.3s | 4 | 16523 | 205 | 4796 | $0.0000 | Edit, Edit.confirm, Read |
| 8 | ✗ FAIL | 8.8s | 8 | 35195 | 244 | 5285 | $0.0000 | Edit×3, Edit.cancel×2, Read×2 |
| 9 | ✗ FAIL | 4.1s | 3 | 11549 | 128 | 4152 | $0.0000 | Edit, Edit.confirm |
| 10 | ✓ PASS | 10.3s | 5 | 22730 | 279 | 5565 | $0.0000 | Edit, Edit.confirm, Read, Search |

**Failures:**

**Run 1**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,7 @@
+function newFunc(): string | null {
+    return null;
+}
+
 function main() {
     const result = newFunc();
     if (newFunc() !== null) {
@@ -6,7 +10,3 @@
     const value = newFunc();
 }
 
-function newFunc(): string | null {
-    return null;
-}
-

```


**Run 3**: 
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
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
+function newFunc() {
+    console.log('newFunc');
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
+    return true;
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

**✗ Context-Specific Replace** | Replace with context for uniqueness | 20% (2/10) | 3.5 calls | 13311 tokens | 12.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 5.8s | 3 | 11186 | 86 | 3896 | $0.0000 | Edit, Edit.confirm |
| 2 | ✗ FAIL | 4.1s | 3 | 11143 | 101 | 3882 | $0.0000 | Edit, Edit.confirm |
| 3 | ✗ FAIL | 9.5s | 2 | 7098 | 58 | 3590 | $0.0000 | Search |
| 4 | ✓ PASS | 27.7s | 6 | 23371 | 153 | 4408 | $0.0000 | Edit, Edit.confirm, Read×3 |
| 5 | ✓ PASS | 57.4s | 6 | 24623 | 343 | 4841 | $0.0000 | Edit, Edit.confirm, Read, Search×2 |
| 6 | ✗ FAIL | 2.7s | 3 | 11166 | 78 | 3890 | $0.0000 | Edit, Edit.confirm |
| 7 | ✗ FAIL | 3.0s | 3 | 11129 | 87 | 3868 | $0.0000 | Edit, Edit.confirm |
| 8 | ✗ FAIL | 3.7s | 3 | 11133 | 91 | 3872 | $0.0000 | Edit, Edit.confirm |
| 9 | ✗ FAIL | 3.1s | 3 | 11128 | 86 | 3867 | $0.0000 | Edit, Edit.confirm |
| 10 | ✗ FAIL | 3.1s | 3 | 11132 | 90 | 3871 | $0.0000 | Edit, Edit.confirm |

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

**✗ Multi-line Block Replace** | Replace multi-line block | 0% (0/10) | 3.0 calls | 11223 tokens | 7.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 5.1s | 3 | 11219 | 89 | 3912 | $0.0000 | Edit, Edit.confirm |
| 2 | ✗ FAIL | 4.3s | 3 | 11219 | 89 | 3912 | $0.0000 | Edit, Edit.confirm |
| 3 | ✗ FAIL | 19.3s | 3 | 11237 | 107 | 3930 | $0.0000 | Edit, Edit.confirm |
| 4 | ✗ FAIL | 16.3s | 3 | 11229 | 91 | 3916 | $0.0000 | Edit, Edit.confirm |
| 5 | ✗ FAIL | 16.1s | 3 | 11219 | 89 | 3912 | $0.0000 | Edit, Edit.confirm |
| 6 | ✗ FAIL | 3.1s | 3 | 11219 | 89 | 3912 | $0.0000 | Edit, Edit.confirm |
| 7 | ✗ FAIL | 3.2s | 3 | 11219 | 89 | 3912 | $0.0000 | Edit, Edit.confirm |
| 8 | ✗ FAIL | 3.1s | 3 | 11219 | 89 | 3912 | $0.0000 | Edit, Edit.confirm |
| 9 | ✗ FAIL | 3.1s | 3 | 11219 | 89 | 3912 | $0.0000 | Edit, Edit.confirm |
| 10 | ✗ FAIL | 3.0s | 3 | 11228 | 87 | 3920 | $0.0000 | Edit, Edit.confirm |

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

**✗ No Match Handling** | Handle search text not found | 20% (2/10) | 3.4 calls | 12739 tokens | 8.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 5.7s | 3 | 11203 | 100 | 3930 | $0.0000 | Edit, Edit.confirm |
| 2 | ✗ FAIL | 5.6s | 4 | 15151 | 126 | 4034 | $0.0000 | Edit, Edit.confirm, Read |
| 3 | ✓ PASS | 10.8s | 3 | 10928 | 61 | 3750 | $0.0000 | Read, Search |
| 4 | ✗ FAIL | 22.7s | 4 | 15313 | 128 | 4145 | $0.0000 | Edit, Edit.confirm, Read |
| 5 | ✗ FAIL | 23.0s | 4 | 15153 | 128 | 4036 | $0.0000 | Edit, Edit.confirm, Read |
| 6 | ✗ FAIL | 4.3s | 4 | 15156 | 132 | 4038 | $0.0000 | Edit, Edit.confirm, Read |
| 7 | ✗ FAIL | 3.1s | 3 | 11195 | 92 | 3922 | $0.0000 | Edit, Edit.confirm |
| 8 | ✓ PASS | 2.4s | 3 | 10942 | 75 | 3765 | $0.0000 | Read, Search |
| 9 | ✗ FAIL | 2.8s | 3 | 11176 | 73 | 3903 | $0.0000 | Edit, Edit.confirm |
| 10 | ✗ FAIL | 2.8s | 3 | 11176 | 73 | 3903 | $0.0000 | Edit, Edit.confirm |

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

output contains 'replaced' but should not
output contains 'successfully' but should not

**Run 2**: output contains 'successfully' but should not

**Run 4**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,4 +1,4 @@
-This is a sample file.
+This is a replacement file.
 It has multiple lines.
 Nothing special here.
 

```

output contains 'replaced' but should not
output contains 'successfully' but should not

**Run 5**: output contains 'successfully' but should not

**Run 6**: output contains 'successfully' but should not

**Run 7**: 
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

**Run 9**: 
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

**Run 10**: 
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

---

#### E9

**✗ Empty Content** | Handle empty replacement | 20% (2/10) | 3.4 calls | 13079 tokens | 8.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 8.2s | 4 | 15636 | 125 | 4298 | $0.0000 | Edit, Edit.confirm, Read |
| 2 | ✗ FAIL | 5.0s | 3 | 11310 | 81 | 3987 | $0.0000 | Edit, Edit.confirm |
| 3 | ✗ FAIL | 15.7s | 2 | 7354 | 91 | 3818 | $0.0000 | Edit |
| 4 | ✓ PASS | 24.9s | 4 | 15630 | 132 | 4291 | $0.0000 | Edit, Edit.confirm, Read |
| 5 | ✗ FAIL | 15.6s | 3 | 11319 | 86 | 3992 | $0.0000 | Edit, Edit.confirm |
| 6 | ✗ FAIL | 4.4s | 4 | 15640 | 129 | 4302 | $0.0000 | Edit, Edit.confirm, Read |
| 7 | ✗ FAIL | 4.2s | 4 | 15630 | 119 | 4292 | $0.0000 | Edit, Edit.confirm, Read |
| 8 | ✗ FAIL | 3.2s | 3 | 11328 | 95 | 4001 | $0.0000 | Edit, Edit.confirm |
| 9 | ✓ PASS | 4.3s | 4 | 15631 | 133 | 4291 | $0.0000 | Edit, Edit.confirm, Read |
| 10 | ✗ FAIL | 2.9s | 3 | 11316 | 83 | 3989 | $0.0000 | Edit, Edit.confirm |

**Failures:**

**Run 1**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,6 @@
 LINE B
 LINE C
 LINE D
-
 LINE F
 LINE G
 LINE H

```


**Run 2**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -1,8 +1,7 @@
 LINE A
 LINE B
-LINE C
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


**Run 5**: 
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


**Run 6**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,6 @@
 LINE B
 LINE C
 LINE D
-
 LINE F
 LINE G
 LINE H

```


**Run 7**: 
```diff
file numbered.txt content does not match expected:
--- expected
+++ actual
@@ -2,7 +2,6 @@
 LINE B
 LINE C
 LINE D
-
 LINE F
 LINE G
 LINE H

```


**Run 8**: 
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


**Run 10**: 
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

**✗ Special Characters** | Handle special characters in replacement | 0% (0/10) | 3.0 calls | 11312 tokens | 8.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 6.1s | 3 | 11310 | 105 | 3984 | $0.0000 | Edit, Edit.confirm |
| 2 | ✗ FAIL | 4.6s | 3 | 11273 | 104 | 3969 | $0.0000 | Edit, Edit.confirm |
| 3 | ✗ FAIL | 21.1s | 3 | 11328 | 123 | 4002 | $0.0000 | Edit, Edit.confirm |
| 4 | ✗ FAIL | 19.2s | 3 | 11307 | 102 | 3981 | $0.0000 | Edit, Edit.confirm |
| 5 | ✗ FAIL | 17.5s | 3 | 11311 | 106 | 3985 | $0.0000 | Edit, Edit.confirm |
| 6 | ✗ FAIL | 4.1s | 3 | 11338 | 133 | 4012 | $0.0000 | Edit, Edit.confirm |
| 7 | ✗ FAIL | 3.7s | 3 | 11319 | 114 | 3993 | $0.0000 | Edit, Edit.confirm |
| 8 | ✗ FAIL | 3.7s | 3 | 11318 | 113 | 3992 | $0.0000 | Edit, Edit.confirm |
| 9 | ✗ FAIL | 3.4s | 3 | 11301 | 96 | 3975 | $0.0000 | Edit, Edit.confirm |
| 10 | ✗ FAIL | 3.5s | 3 | 11310 | 105 | 3984 | $0.0000 | Edit, Edit.confirm |

**Failures:**

**Run 1**: 
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
@@ -1,6 +1,6 @@
-line A
+value = $100 + 50%
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
@@ -1,6 +1,6 @@
-line A
+value = $100 + 50%
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
-line A
+value = $100 + 50%
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
-line A
+value = $100 + 50%
 line B
 line C
-value = $100 + 50%
+placeholder
 line E
 

```


---

#### E11

**✗ Indentation Preservation** | Maintain correct indentation (critical for Python) | 0% (0/10) | 4.0 calls | 15722 tokens | 11.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 8.4s | 4 | 15681 | 134 | 4334 | $0.0000 | Edit, Edit.confirm, Read |
| 2 | ✗ FAIL | 5.7s | 4 | 15677 | 131 | 4328 | $0.0000 | Edit, Edit.confirm, Read |
| 3 | ✗ FAIL | 26.0s | 4 | 15682 | 134 | 4331 | $0.0000 | Edit, Edit.confirm, Read |
| 4 | ✗ FAIL | 21.2s | 3 | 11519 | 114 | 4119 | $0.0000 | Edit, Edit.confirm |
| 5 | ✗ FAIL | 29.5s | 5 | 20773 | 221 | 4885 | $0.0000 | Edit, Edit.confirm, Read, Search |
| 6 | ✗ FAIL | 5.5s | 4 | 15686 | 138 | 4335 | $0.0000 | Edit, Edit.confirm, Read |
| 7 | ✗ FAIL | 3.4s | 3 | 11501 | 96 | 4101 | $0.0000 | Edit, Edit.confirm |
| 8 | ✗ FAIL | 3.3s | 3 | 11505 | 94 | 4103 | $0.0000 | Edit, Edit.confirm |
| 9 | ✗ FAIL | 7.0s | 6 | 23521 | 209 | 4516 | $0.0000 | Edit, Edit.confirm, Read, Search×2 |
| 10 | ✗ FAIL | 4.4s | 4 | 15680 | 134 | 4331 | $0.0000 | Edit, Edit.confirm, Read |

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
         process_data()
-        validate_input()
-        transform_data()
+validate_input()
+transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

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
+validate_input()
+transform_data()
         finalize()
 

```

command failed: exit status 1
output: Sorry: IndentationError: unexpected indent (process.py, line 6)

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
+        # placeholder
         finalize()
-
+validate_input()
+transform_data()

```


**Run 5**: 
```diff
file process.py content does not match expected:
--- expected
+++ actual
@@ -3,5 +3,6 @@
         process_data()
         validate_input()
         transform_data()
+        # placeholder
         finalize()
 

```


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
-        finalize()
+validate_input()
+transform_data()
+finalize()
 

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

**✓ Simple File Read** | Read an entire small file | 100% (10/10) | 2.0 calls | 7195 tokens | 4.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 3.3s | 2 | 7190 | 60 | 3695 | $0.0000 | Read |
| 2 | ✓ PASS | 3.9s | 2 | 7190 | 60 | 3695 | $0.0000 | Read |
| 3 | ✓ PASS | 2.6s | 2 | 7190 | 60 | 3695 | $0.0000 | Read |
| 4 | ✓ PASS | 11.1s | 2 | 7200 | 70 | 3705 | $0.0000 | Read |
| 5 | ✓ PASS | 10.8s | 2 | 7197 | 67 | 3702 | $0.0000 | Read |
| 6 | ✓ PASS | 2.8s | 2 | 7190 | 60 | 3695 | $0.0000 | Read |
| 7 | ✓ PASS | 3.0s | 2 | 7201 | 71 | 3706 | $0.0000 | Read |
| 8 | ✓ PASS | 2.0s | 2 | 7201 | 71 | 3706 | $0.0000 | Read |
| 9 | ✓ PASS | 3.1s | 2 | 7190 | 60 | 3695 | $0.0000 | Read |
| 10 | ✓ PASS | 2.0s | 2 | 7201 | 71 | 3706 | $0.0000 | Read |

---

#### R2

**✗ Truncation Recovery** | Handle truncated output by chunked reading | 70% (7/10) | 2.0 calls | 7850 tokens | 6.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.5s | 2 | 7844 | 67 | 4341 | $0.0000 | Read |
| 2 | ✓ PASS | 4.7s | 2 | 7857 | 80 | 4354 | $0.0000 | Read |
| 3 | ✓ PASS | 4.0s | 2 | 7859 | 82 | 4356 | $0.0000 | Read |
| 4 | ✓ PASS | 22.3s | 2 | 7891 | 114 | 4388 | $0.0000 | Read |
| 5 | ✓ PASS | 17.9s | 2 | 7861 | 84 | 4358 | $0.0000 | Read |
| 6 | ✗ FAIL | 1.8s | 2 | 7822 | 45 | 4319 | $0.0000 | Read |
| 7 | ✓ PASS | 2.7s | 2 | 7871 | 94 | 4368 | $0.0000 | Read |
| 8 | ✓ PASS | 2.5s | 2 | 7860 | 83 | 4357 | $0.0000 | Read |
| 9 | ✗ FAIL | 1.6s | 2 | 7819 | 42 | 4316 | $0.0000 | Read |
| 10 | ✗ FAIL | 1.6s | 2 | 7818 | 41 | 4315 | $0.0000 | Read |

**Failures:**

**Run 6**: output does not contain 'SECTION_ALPHA'
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

**✓ Relative vs Absolute Path** | Correct path resolution | 100% (10/10) | 2.0 calls | 7162 tokens | 3.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.1s | 2 | 7160 | 46 | 3659 | $0.0000 | Read |
| 2 | ✓ PASS | 2.9s | 2 | 7162 | 48 | 3661 | $0.0000 | Read |
| 3 | ✓ PASS | 2.0s | 2 | 7160 | 46 | 3659 | $0.0000 | Read |
| 4 | ✓ PASS | 7.5s | 2 | 7161 | 47 | 3660 | $0.0000 | Read |
| 5 | ✓ PASS | 7.7s | 2 | 7160 | 46 | 3659 | $0.0000 | Read |
| 6 | ✓ PASS | 1.5s | 2 | 7160 | 47 | 3659 | $0.0000 | Read |
| 7 | ✓ PASS | 1.5s | 2 | 7160 | 46 | 3659 | $0.0000 | Read |
| 8 | ✓ PASS | 1.5s | 2 | 7160 | 46 | 3659 | $0.0000 | Read |
| 9 | ✓ PASS | 1.8s | 2 | 7175 | 62 | 3674 | $0.0000 | Read |
| 10 | ✓ PASS | 1.6s | 2 | 7161 | 48 | 3660 | $0.0000 | Read |

---

#### S1

**✗ Simple Pattern Search** | Find a specific function definition | 10% (1/10) | 3.4 calls | 13101 tokens | 10.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 3.7s | 2 | 7111 | 72 | 3608 | $0.0000 | Search |
| 2 | ✗ FAIL | 11.7s | 4 | 15656 | 184 | 4370 | $0.0000 | Read×2, Search |
| 3 | ✗ FAIL | 9.6s | 4 | 15688 | 213 | 4397 | $0.0000 | Read×2, Search |
| 4 | ✗ FAIL | 34.7s | 4 | 15596 | 201 | 4355 | $0.0000 | Read×2, Search |
| 5 | ✗ FAIL | 7.9s | 2 | 7089 | 50 | 3586 | $0.0000 | Search |
| 6 | ✓ PASS | 10.0s | 4 | 15834 | 359 | 4543 | $0.0000 | Read×2, Search |
| 7 | ✗ FAIL | 6.7s | 4 | 15605 | 210 | 4364 | $0.0000 | Read×2, Search |
| 8 | ✗ FAIL | 7.0s | 4 | 15680 | 208 | 4394 | $0.0000 | Read×2, Search |
| 9 | ✗ FAIL | 1.6s | 2 | 7091 | 52 | 3588 | $0.0000 | Search |
| 10 | ✗ FAIL | 7.7s | 4 | 15660 | 188 | 4374 | $0.0000 | Read×2, Search |

**Failures:**

**Run 1**: output does not contain 'func calculateTotal(items []int)'

**Run 2**: output does not contain 'func calculateTotal(items []int)'

**Run 3**: output does not contain 'func calculateTotal(items []int)'

**Run 4**: output does not contain 'func calculateTotal(items []int)'

**Run 5**: output does not contain 'func calculateTotal(items []int)'

**Run 7**: output does not contain 'func calculateTotal(items []int)'

**Run 8**: output does not contain 'func calculateTotal(items []int)'

**Run 9**: output does not contain 'func calculateTotal(items []int)'

**Run 10**: output does not contain 'func calculateTotal(items []int)'

---

#### S2

**✗ Multi-Pattern Search** | Find multiple related items | 90% (9/10) | 2.0 calls | 7207 tokens | 8.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 6.6s | 2 | 7232 | 125 | 3643 | $0.0000 | Shell |
| 2 | ✗ FAIL | 2.7s | 2 | 7075 | 45 | 3563 | $0.0000 | Shell |
| 3 | ✓ PASS | 3.4s | 2 | 7124 | 71 | 3589 | $0.0000 | Shell |
| 4 | ✓ PASS | 42.6s | 2 | 7490 | 254 | 3772 | $0.0000 | Shell |
| 5 | ✓ PASS | 11.2s | 2 | 7124 | 71 | 3589 | $0.0000 | Shell |
| 6 | ✓ PASS | 5.8s | 2 | 7304 | 161 | 3679 | $0.0000 | Shell |
| 7 | ✓ PASS | 4.3s | 2 | 7220 | 119 | 3637 | $0.0000 | Shell |
| 8 | ✓ PASS | 4.3s | 2 | 7218 | 117 | 3635 | $0.0000 | Shell |
| 9 | ✓ PASS | 2.7s | 2 | 7134 | 76 | 3594 | $0.0000 | Shell |
| 10 | ✓ PASS | 3.0s | 2 | 7148 | 83 | 3601 | $0.0000 | Shell |

**Failures:**

**Run 2**: output does not contain '7'

---

#### S3

**✗ Search with File Filtering** | Search in specific file types | 0% (0/10) | 2.0 calls | 7113 tokens | 2.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 2.3s | 2 | 7123 | 45 | 3589 | $0.0000 | Search |
| 2 | ✗ FAIL | 2.4s | 2 | 7108 | 37 | 3582 | $0.0000 | Search |
| 3 | ✗ FAIL | 2.1s | 2 | 7117 | 46 | 3591 | $0.0000 | Search |
| 4 | ✗ FAIL | 6.1s | 2 | 7107 | 36 | 3581 | $0.0000 | Search |
| 5 | ✗ FAIL | 5.8s | 2 | 7108 | 37 | 3582 | $0.0000 | Search |
| 6 | ✗ FAIL | 1.6s | 2 | 7117 | 46 | 3591 | $0.0000 | Search |
| 7 | ✗ FAIL | 1.5s | 2 | 7113 | 42 | 3587 | $0.0000 | Search |
| 8 | ✗ FAIL | 1.4s | 2 | 7108 | 37 | 3582 | $0.0000 | Search |
| 9 | ✗ FAIL | 1.4s | 2 | 7108 | 37 | 3582 | $0.0000 | Search |
| 10 | ✗ FAIL | 1.6s | 2 | 7117 | 46 | 3591 | $0.0000 | Search |

**Failures:**

**Run 1**: output does not contain 'models.go'

**Run 2**: output does not contain 'models.go'

**Run 3**: output does not contain 'models.go'

**Run 4**: output does not contain 'models.go'

**Run 5**: output does not contain 'models.go'

**Run 6**: output does not contain 'models.go'

**Run 7**: output does not contain 'models.go'

**Run 8**: output does not contain 'models.go'

**Run 9**: output does not contain 'models.go'

**Run 10**: output does not contain 'models.go'

---

#### S4

**✗ Search Truncation Recovery** | Handle large search results requiring refinement | 30% (3/10) | 3.3 calls | 15630 tokens | 8.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 6.8s | 2 | 7218 | 118 | 3639 | $0.0000 | Shell |
| 2 | ✗ FAIL | 5.0s | 2 | 8961 | 52 | 5464 | $0.0000 | Search |
| 3 | ✗ FAIL | 15.4s | 4 | 17767 | 335 | 6660 | $0.0000 | Read, Shell×2 |
| 4 | ✗ FAIL | 8.2s | 2 | 7085 | 54 | 3574 | $0.0000 | Shell |
| 5 | ✗ FAIL | 13.9s | 2 | 7174 | 90 | 3639 | $0.0000 | Shell |
| 6 | ✗ FAIL | 5.2s | 3 | 10665 | 156 | 3595 | $0.0000 | Shell |
| 7 | ✗ FAIL | 2.1s | 2 | 8954 | 43 | 5457 | $0.0000 | Search |
| 8 | ✗ FAIL | 10.3s | 6 | 23322 | 407 | 4265 | $0.0000 | Shell×5 |
| 9 | ✓ PASS | 13.1s | 7 | 50529 | 392 | 11691 | $0.0000 | Read×4, Search×2 |
| 10 | ✓ PASS | 3.6s | 3 | 14623 | 106 | 5651 | $0.0000 | Read, Search |

**Failures:**

**Run 2**: output does not contain 'folder3'

**Run 3**: output does not contain 'folder3'

**Run 4**: output does not contain 'folder3'

**Run 5**: output does not contain 'folder3'

**Run 6**: output does not contain 'folder3'

**Run 7**: output does not contain 'folder3'

**Run 8**: output does not contain 'folder3'

---

#### W1

**✓ Simple File Creation** | Create a new file with content | 100% (10/10) | 2.0 calls | 7107 tokens | 3.5s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 3.5s | 2 | 7111 | 61 | 3602 | $0.0000 | Write |
| 2 | ✓ PASS | 3.5s | 2 | 7107 | 57 | 3598 | $0.0000 | Write |
| 3 | ✓ PASS | 2.3s | 2 | 7099 | 49 | 3590 | $0.0000 | Write |
| 4 | ✓ PASS | 9.3s | 2 | 7106 | 56 | 3597 | $0.0000 | Write |
| 5 | ✓ PASS | 7.7s | 2 | 7099 | 49 | 3590 | $0.0000 | Write |
| 6 | ✓ PASS | 1.8s | 2 | 7113 | 63 | 3604 | $0.0000 | Write |
| 7 | ✓ PASS | 1.9s | 2 | 7113 | 63 | 3604 | $0.0000 | Write |
| 8 | ✓ PASS | 1.9s | 2 | 7113 | 63 | 3604 | $0.0000 | Write |
| 9 | ✓ PASS | 1.6s | 2 | 7099 | 49 | 3590 | $0.0000 | Write |
| 10 | ✓ PASS | 1.8s | 2 | 7111 | 61 | 3602 | $0.0000 | Write |

---

#### W2

**✓ Multi-line Content** | Write file with multiple lines and formatting | 100% (10/10) | 2.0 calls | 7163 tokens | 5.1s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.4s | 2 | 7164 | 83 | 3628 | $0.0000 | Write |
| 2 | ✓ PASS | 4.7s | 2 | 7166 | 85 | 3630 | $0.0000 | Write |
| 3 | ✓ PASS | 3.2s | 2 | 7158 | 77 | 3622 | $0.0000 | Write |
| 4 | ✓ PASS | 13.1s | 2 | 7166 | 85 | 3630 | $0.0000 | Write |
| 5 | ✓ PASS | 13.2s | 2 | 7168 | 87 | 3632 | $0.0000 | Write |
| 6 | ✓ PASS | 2.4s | 2 | 7153 | 72 | 3617 | $0.0000 | Write |
| 7 | ✓ PASS | 2.5s | 2 | 7158 | 77 | 3622 | $0.0000 | Write |
| 8 | ✓ PASS | 2.5s | 2 | 7158 | 77 | 3622 | $0.0000 | Write |
| 9 | ✓ PASS | 2.5s | 2 | 7158 | 77 | 3622 | $0.0000 | Write |
| 10 | ✓ PASS | 3.0s | 2 | 7182 | 101 | 3646 | $0.0000 | Write |

---

#### W3

**✗ Overwrite Existing** | Handle overwrite of existing file | 90% (9/10) | 2.9 calls | 10488 tokens | 4.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.1s | 3 | 10861 | 63 | 3718 | $0.0000 | Write, Write.confirm |
| 2 | ✓ PASS | 3.7s | 3 | 10859 | 61 | 3716 | $0.0000 | Write, Write.confirm |
| 3 | ✓ PASS | 3.0s | 3 | 10859 | 61 | 3716 | $0.0000 | Write, Write.confirm |
| 4 | ✓ PASS | 11.6s | 3 | 10861 | 63 | 3718 | $0.0000 | Write, Write.confirm |
| 5 | ✓ PASS | 10.2s | 3 | 10859 | 61 | 3716 | $0.0000 | Write, Write.confirm |
| 6 | ✓ PASS | 2.2s | 3 | 10859 | 61 | 3716 | $0.0000 | Write, Write.confirm |
| 7 | ✗ FAIL | 1.5s | 2 | 7141 | 44 | 3634 | $0.0000 | Write |
| 8 | ✓ PASS | 2.2s | 3 | 10859 | 61 | 3716 | $0.0000 | Write, Write.confirm |
| 9 | ✓ PASS | 2.2s | 3 | 10859 | 61 | 3716 | $0.0000 | Write, Write.confirm |
| 10 | ✓ PASS | 2.3s | 3 | 10861 | 63 | 3718 | $0.0000 | Write, Write.confirm |

**Failures:**

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


---

#### W4

**✗ Special Characters** | Handle special characters in content | 90% (9/10) | 2.0 calls | 7136 tokens | 3.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.9s | 2 | 7131 | 52 | 3604 | $0.0000 | Write |
| 2 | ✓ PASS | 3.3s | 2 | 7132 | 53 | 3605 | $0.0000 | Write |
| 3 | ✓ PASS | 3.1s | 2 | 7159 | 80 | 3632 | $0.0000 | Write |
| 4 | ✓ PASS | 8.8s | 2 | 7131 | 52 | 3604 | $0.0000 | Write |
| 5 | ✗ FAIL | 8.8s | 2 | 7130 | 52 | 3603 | $0.0000 | Write |
| 6 | ✓ PASS | 2.2s | 2 | 7153 | 74 | 3626 | $0.0000 | Write |
| 7 | ✓ PASS | 1.8s | 2 | 7132 | 53 | 3605 | $0.0000 | Write |
| 8 | ✓ PASS | 1.8s | 2 | 7132 | 53 | 3605 | $0.0000 | Write |
| 9 | ✓ PASS | 1.8s | 2 | 7131 | 52 | 3604 | $0.0000 | Write |
| 10 | ✓ PASS | 1.8s | 2 | 7131 | 52 | 3604 | $0.0000 | Write |

**Failures:**

**Run 5**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-'quotes', "double", `backticks`, $var
-
+quotes, "double", `backticks`, $var

```


---

#### W5

**✓ Empty File Creation** | Create an empty file | 100% (10/10) | 2.0 calls | 7078 tokens | 2.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.4s | 2 | 7083 | 47 | 3583 | $0.0000 | Write |
| 2 | ✓ PASS | 2.8s | 2 | 7085 | 49 | 3585 | $0.0000 | Write |
| 3 | ✓ PASS | 1.8s | 2 | 7073 | 37 | 3573 | $0.0000 | Write |
| 4 | ✓ PASS | 7.5s | 2 | 7083 | 47 | 3583 | $0.0000 | Write |
| 5 | ✓ PASS | 5.9s | 2 | 7073 | 37 | 3573 | $0.0000 | Write |
| 6 | ✓ PASS | 1.2s | 2 | 7072 | 36 | 3572 | $0.0000 | Write |
| 7 | ✓ PASS | 1.3s | 2 | 7073 | 37 | 3573 | $0.0000 | Write |
| 8 | ✓ PASS | 1.4s | 2 | 7079 | 43 | 3579 | $0.0000 | Write |
| 9 | ✓ PASS | 1.5s | 2 | 7084 | 48 | 3584 | $0.0000 | Write |
| 10 | ✓ PASS | 1.4s | 2 | 7079 | 43 | 3579 | $0.0000 | Write |

---

#### W6

**✓ Path with Spaces** | Handle paths with spaces | 100% (10/10) | 2.0 calls | 7120 tokens | 3.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.7s | 2 | 7116 | 51 | 3600 | $0.0000 | Write |
| 2 | ✓ PASS | 2.9s | 2 | 7116 | 51 | 3600 | $0.0000 | Write |
| 3 | ✓ PASS | 2.4s | 2 | 7117 | 52 | 3601 | $0.0000 | Write |
| 4 | ✓ PASS | 8.0s | 2 | 7116 | 51 | 3600 | $0.0000 | Write |
| 5 | ✓ PASS | 8.2s | 2 | 7117 | 52 | 3601 | $0.0000 | Write |
| 6 | ✓ PASS | 1.6s | 2 | 7116 | 51 | 3600 | $0.0000 | Write |
| 7 | ✓ PASS | 1.9s | 2 | 7124 | 59 | 3608 | $0.0000 | Write |
| 8 | ✓ PASS | 2.2s | 2 | 7143 | 78 | 3627 | $0.0000 | Write |
| 9 | ✓ PASS | 1.7s | 2 | 7116 | 51 | 3600 | $0.0000 | Write |
| 10 | ✓ PASS | 1.7s | 2 | 7116 | 51 | 3600 | $0.0000 | Write |

---

## Failure Analysis

| Benchmark | Run | Errors | Last Tool Call |
|-----------|-----|--------|----------------|
| S1 | 1 | output does not contain 'func calculateTotal(it... | Search |
| S3 | 1 | output does not contain 'models.go' | Search |
| E2 | 1 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 1 | file boundary.txt content does not match expect... | Edit.confirm |
| E5 | 1 | file code.ts content does not match expected:
-... | Edit.confirm |
| E6 | 1 | file values.ts content does not match expected:... | Edit.confirm |
| E7 | 1 | file func.ts content does not match expected:
-... | Edit.confirm |
| E8 | 1 | file sample.txt content does not match expected... | Edit.confirm |
| E9 | 1 | file numbered.txt content does not match expect... | Edit.confirm |
| E10 | 1 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 1 | file process.py content does not match expected... | Edit.confirm |
| C1 | 1 | output does not contain 'db.example.com' | Search |
| C5 | 1 | output does not contain 'wire.go' | Search |
| S1 | 2 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 2 | output does not contain '7' | Shell |
| S3 | 2 | output does not contain 'models.go' | Search |
| S4 | 2 | output does not contain 'folder3' | Search |
| E2 | 2 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 2 | file boundary.txt content does not match expect... | Edit.confirm |
| E6 | 2 | file values.ts content does not match expected:... | Edit.confirm |
| E7 | 2 | file func.ts content does not match expected:
-... | Edit.confirm |
| E8 | 2 | output contains 'successfully' but should not | Edit.confirm |
| E9 | 2 | file numbered.txt content does not match expect... | Edit.confirm |
| E10 | 2 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 2 | file process.py content does not match expected... | Edit.confirm |
| C1 | 2 | output does not contain 'db.example.com' | Search |
| C2 | 2 | file config.yaml content does not match expecte... |  |
| S1 | 3 | output does not contain 'func calculateTotal(it... | Read |
| S3 | 3 | output does not contain 'models.go' | Search |
| S4 | 3 | output does not contain 'folder3' | Read |
| E4 | 3 | file boundary.txt content does not match expect... | Edit.confirm |
| E5 | 3 | file code.ts content does not match expected:
-... | Edit.confirm |
| E6 | 3 | file values.ts content does not match expected:... | Search |
| E7 | 3 | file func.ts content does not match expected:
-... | Edit.confirm |
| E9 | 3 | file numbered.txt content does not match expect... | Edit |
| E10 | 3 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 3 | file process.py content does not match expected... | Edit.confirm |
| C1 | 3 | output does not contain 'db.example.com' | Search |
| C5 | 3 | output does not contain 'wire.go' | Search |
| S1 | 4 | output does not contain 'func calculateTotal(it... | Read |
| S3 | 4 | output does not contain 'models.go' | Search |
| S4 | 4 | output does not contain 'folder3' | Shell |
| E2 | 4 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 4 | file boundary.txt content does not match expect... | Edit.confirm |
| E5 | 4 | file code.ts content does not match expected:
-... | Edit |
| E7 | 4 | file func.ts content does not match expected:
-... | Edit.confirm |
| E8 | 4 | file sample.txt content does not match expected... | Edit.confirm |
| E10 | 4 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 4 | file process.py content does not match expected... | Edit.confirm |
| C1 | 4 | output does not contain 'db.example.com' | Search |
| C2 | 4 | file config.yaml content does not match expecte... | Edit.confirm |
| C5 | 4 | output does not contain 'wire.go' | Search |
| S1 | 5 | output does not contain 'func calculateTotal(it... | Search |
| S3 | 5 | output does not contain 'models.go' | Search |
| S4 | 5 | output does not contain 'folder3' | Shell |
| W4 | 5 | file special.txt content does not match expecte... | Write |
| E2 | 5 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 5 | file boundary.txt content does not match expect... | Edit.confirm |
| E5 | 5 | file code.ts content does not match expected:
-... | Edit.confirm |
| E7 | 5 | file func.ts content does not match expected:
-... | Edit.confirm |
| E8 | 5 | output contains 'successfully' but should not | Edit.confirm |
| E9 | 5 | file numbered.txt content does not match expect... | Edit.confirm |
| E10 | 5 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 5 | file process.py content does not match expected... | Edit.confirm |
| C1 | 5 | output does not contain 'db.example.com' | Search |
| C5 | 5 | output does not contain 'wire.go' | Search |
| S3 | 6 | output does not contain 'models.go' | Search |
| S4 | 6 | output does not contain 'folder3' | Shell |
| R2 | 6 | output does not contain 'SECTION_ALPHA'; output... | Read |
| E2 | 6 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 6 | file boundary.txt content does not match expect... | Edit.confirm |
| E6 | 6 | file values.ts content does not match expected:... | Edit.confirm |
| E7 | 6 | file func.ts content does not match expected:
-... | Edit.confirm |
| E8 | 6 | output contains 'successfully' but should not | Edit.confirm |
| E9 | 6 | file numbered.txt content does not match expect... | Edit.confirm |
| E10 | 6 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 6 | file process.py content does not match expected... | Edit.confirm |
| C3 | 6 | file utils.ts content does not match expected:
... | Edit.confirm |
| C5 | 6 | output does not contain 'wire.go' | Search |
| S1 | 7 | output does not contain 'func calculateTotal(it... | Read |
| S3 | 7 | output does not contain 'models.go' | Search |
| S4 | 7 | output does not contain 'folder3' | Search |
| W3 | 7 | file existing.txt content does not match expect... | Write |
| E2 | 7 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 7 | file boundary.txt content does not match expect... | Edit.confirm |
| E6 | 7 | file values.ts content does not match expected:... | Edit.confirm |
| E7 | 7 | file func.ts content does not match expected:
-... | Edit.confirm |
| E8 | 7 | file sample.txt content does not match expected... | Edit.confirm |
| E9 | 7 | file numbered.txt content does not match expect... | Edit.confirm |
| E10 | 7 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 7 | file process.py content does not match expected... | Edit.confirm |
| C1 | 7 | output does not contain 'db.example.com' | Search |
| C2 | 7 | file config.yaml content does not match expecte... |  |
| C5 | 7 | output does not contain 'wire.go' | Search |
| S1 | 8 | output does not contain 'func calculateTotal(it... | Read |
| S3 | 8 | output does not contain 'models.go' | Search |
| S4 | 8 | output does not contain 'folder3' | Shell |
| E2 | 8 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 8 | file boundary.txt content does not match expect... | Edit.confirm |
| E5 | 8 | file code.ts content does not match expected:
-... | Edit |
| E6 | 8 | file values.ts content does not match expected:... | Edit.confirm |
| E7 | 8 | file func.ts content does not match expected:
-... | Edit.confirm |
| E9 | 8 | file numbered.txt content does not match expect... | Edit.confirm |
| E10 | 8 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 8 | file process.py content does not match expected... | Edit.confirm |
| C1 | 8 | output does not contain 'db.example.com' | Read |
| C2 | 8 | file config.yaml content does not match expecte... | Edit.confirm |
| C5 | 8 | output does not contain 'wire.go' | Search |
| S1 | 9 | output does not contain 'func calculateTotal(it... | Search |
| S3 | 9 | output does not contain 'models.go' | Search |
| R2 | 9 | output does not contain 'SECTION_ALPHA'; output... | Read |
| E2 | 9 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 9 | file boundary.txt content does not match expect... | Edit.confirm |
| E5 | 9 | file code.ts content does not match expected:
-... | Edit.confirm |
| E6 | 9 | file values.ts content does not match expected:... | Edit.confirm |
| E7 | 9 | file func.ts content does not match expected:
-... | Edit.confirm |
| E8 | 9 | file sample.txt content does not match expected... | Edit.confirm |
| E10 | 9 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 9 | file process.py content does not match expected... | Edit.confirm |
| C1 | 9 | output does not contain 'db.example.com' | Search |
| C3 | 9 | file handler1.ts content does not match expecte... | Edit.confirm |
| S1 | 10 | output does not contain 'func calculateTotal(it... | Read |
| S3 | 10 | output does not contain 'models.go' | Search |
| R2 | 10 | output does not contain 'SECTION_ALPHA'; output... | Read |
| E2 | 10 | file file.txt content does not match expected:
... | Edit.confirm |
| E4 | 10 | file boundary.txt content does not match expect... | Edit.confirm |
| E6 | 10 | file values.ts content does not match expected:... | Edit.confirm |
| E7 | 10 | file func.ts content does not match expected:
-... | Edit.confirm |
| E8 | 10 | file sample.txt content does not match expected... | Edit.confirm |
| E9 | 10 | file numbered.txt content does not match expect... | Edit.confirm |
| E10 | 10 | file special.txt content does not match expecte... | Edit.confirm |
| E11 | 10 | file process.py content does not match expected... | Edit.confirm |
| C1 | 10 | output does not contain 'db.example.com' | Search |
| C5 | 10 | output does not contain 'wire.go' | Search |

## Appendix A: Configuration

### Version

```
kvit-coder baaf1fc (commit 20260103, built 2026-01-03)
```

### config.yaml

```yaml

```

