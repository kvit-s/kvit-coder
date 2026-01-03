# LLM Tool Usage Benchmark Report

## Metadata

- **Version**: kvit-coder de939e8 (commit 20260101, built 2026-01-02)
- **Date**: 2026-01-01T23:55:38-06:00
- **Total Benchmarks**: 28
- **Total Runs**: 280

## Summary

| Class | Success Rate | Avg Time/Run |
|-------|--------------|--------------|
| C | 38% (15/40) | 47.3s |
| E | 26% (29/110) | 52.7s |
| R | 97% (29/30) | 8.7s |
| S | 18% (7/40) | 27.8s |
| W | 97% (58/60) | 14.5s |
| **Total** | **49% (138/280)** | **151.1s** |

## Detailed Statistics

### Per-Benchmark Summary

| Benchmark | Success | LLM Calls | Tokens | Generated | Context | Prompt Speed | Gen Speed | Cost | Duration |
|-----------|---------|-----------|--------|-----------|---------|--------------|-----------|------|----------|
| C1 | 10% | 2.5(±1.2) | 7620(±4062) | 81(±76) | 3106 | 1191.6 t/s | 26.4 t/s | $0.0000 | 3.3s(±2.9) |
| C2 | 70% | 3.6(±1.4) | 11089(±4151) | 120(±54) | 2950 | 1334.3 t/s | 24.7 t/s | $0.0000 | 17.3s(±34.3) |
| C3 | 30% | 7.8(±1.2) | 32077(±6035) | 518(±133) | 5197 | 1548.3 t/s | 26.0 t/s | $0.0000 | 20.9s(±5.0) |
| C5 | 40% | 3.3(±3.9) | 11990(±15437) | 136(±157) | 4125 | 1320.7 t/s | 27.2 t/s | $0.0000 | 5.8s(±6.2) |
| E1 | 100% | 2.0 | 6136(±9) | 81(±9) | 3183 | 1689.2 t/s | 25.4 t/s | $0.0000 | 3.3s(±0.4) |
| E2 | 0% | 3.0 | 9934(±183) | 152(±34) | 3692 | 1714.3 t/s | 24.3 t/s | $0.0000 | 6.6s(±1.6) |
| E3 | 0% | 3.0 | 9776(±127) | 122(±19) | 3573 | 1697.3 t/s | 25.0 t/s | $0.0000 | 5.2s(±0.8) |
| E4 | 0% | 2.0 | 6078(±7) | 81(±7) | 3131 | 1445.8 t/s | 25.6 t/s | $0.0000 | 3.3s(±0.3) |
| E5 | 50% | 4.0(±1.6) | 14377(±7034) | 210(±92) | 3993 | 1482.4 t/s | 24.7 t/s | $0.0000 | 9.2s(±4.0) |
| E6 | 30% | 2.9(±1.1) | 9037(±3799) | 101(±36) | 3263 | 1456.0 t/s | 25.0 t/s | $0.0000 | 4.3s(±1.6) |
| E7 | 0% | 2.4(±1.2) | 7436(±4123) | 94(±61) | 3198 | 1453.7 t/s | 25.3 t/s | $0.0000 | 3.9s(±2.5) |
| E8 | 0% | 2.1(±0.3) | 6383(±980) | 79(±14) | 3133 | 1531.9 t/s | 25.7 t/s | $0.0000 | 3.2s(±0.6) |
| E9 | 100% | 3.0 | 9548(±17) | 114(±17) | 3429 | 1647.6 t/s | 25.2 t/s | $0.0000 | 4.8s(±0.6) |
| E10 | 0% | 2.2(±0.4) | 6691(±1178) | 98(±19) | 3149 | 1522.5 t/s | 25.5 t/s | $0.0000 | 4.0s(±0.8) |
| E11 | 10% | 2.5(±0.7) | 7810(±2147) | 119(±26) | 3303 | 1616.1 t/s | 25.3 t/s | $0.0000 | 4.9s(±1.1) |
| R1 | 90% | 2.0 | 6032(±14) | 68(±14) | 3120 | 1916.8 t/s | 32.2 t/s | $0.0000 | 2.3s(±0.8) |
| R2 | 100% | 2.1(±0.3) | 7100(±1167) | 106(±24) | 3805 | 1439.1 t/s | 26.6 t/s | $0.0000 | 4.4s(±0.9) |
| R3 | 100% | 2.0 | 5997(±6) | 50(±7) | 3079 | 1532.6 t/s | 26.3 t/s | $0.0000 | 2.0s(±0.2) |
| S1 | 10% | 3.4(±0.9) | 11114(±3396) | 162(±76) | 3565 | 2699.5 t/s | 41.7 t/s | $0.0000 | 4.2s(±2.0) |
| S2 | 40% | 1.8(±0.6) | 5448(±1821) | 107(±75) | 2748 | 2189.0 t/s | 37.0 t/s | $0.0000 | 14.9s(±35.1) |
| S3 | 10% | 2.0 | 5993(±99) | 48(±15) | 3043 | 2080.5 t/s | 39.6 t/s | $0.0000 | 1.3s(±0.4) |
| S4 | 10% | 3.3(±2.1) | 23418(±42555) | 249(±371) | 5563 | 1974.5 t/s | 40.8 t/s | $0.0000 | 7.4s(±11.7) |
| W1 | 100% | 2.0 | 5939(±5) | 55(±5) | 3013 | 1116.7 t/s | 26.4 t/s | $0.0000 | 2.2s(±0.2) |
| W2 | 100% | 2.0 | 6000(±10) | 85(±10) | 3047 | 1240.6 t/s | 25.4 t/s | $0.0000 | 3.3s(±0.4) |
| W3 | 90% | 2.9(±0.3) | 8802(±932) | 65(±5) | 3130 | 1214.4 t/s | 25.5 t/s | $0.0000 | 2.8s(±0.3) |
| W4 | 90% | 2.0 | 5970(±13) | 57(±13) | 3026 | 1200.9 t/s | 25.4 t/s | $0.0000 | 2.3s(±0.4) |
| W5 | 100% | 2.0 | 5912(±5) | 42(±5) | 2995 | 1099.8 t/s | 26.3 t/s | $0.0000 | 1.7s(±0.2) |
| W6 | 100% | 2.0 | 5950(±1) | 51(±1) | 3017 | 1132.6 t/s | 25.9 t/s | $0.0000 | 2.2s(±0.0) |

### Per-Benchmark Details

#### C1

**✗ Search Then Read** | Find and read a file | 10% (1/10) | 2.5 calls | 7620 tokens | 3.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.5s | 2 | 5898 | 32 | 2977 | $0.0000 | Search |
| 2 | ✗ FAIL | 5.1s | 3 | 9502 | 121 | 3435 | $0.0000 | Read, Search |
| 3 | ✗ FAIL | 1.7s | 2 | 5905 | 39 | 2984 | $0.0000 | Search |
| 4 | ✗ FAIL | 2.2s | 2 | 5921 | 55 | 3000 | $0.0000 | Search |
| 5 | ✓ PASS | 11.4s | 6 | 19376 | 300 | 3673 | $0.0000 | Read×2, Search×3 |
| 6 | ✗ FAIL | 2.1s | 2 | 5918 | 52 | 2997 | $0.0000 | Search |
| 7 | ✗ FAIL | 2.3s | 2 | 5924 | 58 | 3003 | $0.0000 | Search |
| 8 | ✗ FAIL | 2.0s | 2 | 5914 | 48 | 2993 | $0.0000 | Search |
| 9 | ✗ FAIL | 2.0s | 2 | 5914 | 48 | 2993 | $0.0000 | Search |
| 10 | ✗ FAIL | 2.4s | 2 | 5926 | 60 | 3005 | $0.0000 | Search |

**Failures:**

**Run 1**: output does not contain 'db.example.com'

**Run 2**: output does not contain 'db.example.com'

**Run 3**: output does not contain 'db.example.com'

**Run 4**: output does not contain 'db.example.com'

**Run 6**: output does not contain 'db.example.com'

**Run 7**: output does not contain 'db.example.com'

**Run 8**: output does not contain 'db.example.com'

**Run 9**: output does not contain 'db.example.com'

**Run 10**: output does not contain 'db.example.com'

---

#### C2

**✗ Read-Modify-Write** | Complete edit workflow | 70% (7/10) | 3.6 calls | 11089 tokens | 17.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.7s | 4 | 12330 | 107 | 3238 | $0.0000 | Shell, Write, Write.confirm |
| 2 | ✓ PASS | 7.6s | 5 | 15277 | 156 | 3239 | $0.0000 | Shell, Write, Write.confirm |
| 3 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 4 | ✗ FAIL | 5.8s | 3 | 9423 | 114 | 3361 | $0.0000 | Edit, Read |
| 5 | ✓ PASS | 4.7s | 4 | 12330 | 107 | 3238 | $0.0000 | Shell, Write, Write.confirm |
| 6 | ✓ PASS | 9.0s | 5 | 15270 | 212 | 3316 | $0.0000 | Edit, Read |
| 7 | ✓ PASS | 4.6s | 4 | 12330 | 107 | 3238 | $0.0000 | Shell, Write, Write.confirm |
| 8 | ✓ PASS | 4.5s | 3 | 9180 | 106 | 3222 | $0.0000 | Edit, Shell |
| 9 | ✗ FAIL | 7.6s | 4 | 12419 | 184 | 3407 | $0.0000 | Edit, Read |
| 10 | ✓ PASS | 4.9s | 4 | 12330 | 107 | 3238 | $0.0000 | Shell, Write, Write.confirm |

**Failures:**

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
-server:
-  host: localhost
-  port: 8080
+    server:
+      host: localhost
+      port: 8080
 

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

**✗ Search-Read-Edit** | Find, understand, and modify | 30% (3/10) | 7.8 calls | 32077 tokens | 20.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 23.7s | 10 | 44152 | 554 | 5773 | $0.0000 | Edit×3, Read×4, Search×2 |
| 2 | ✗ FAIL | 22.8s | 8 | 33052 | 585 | 5317 | $0.0000 | Edit×3, Read×3, Search |
| 3 | ✓ PASS | 21.3s | 8 | 32807 | 550 | 5181 | $0.0000 | Edit×3, Read×3, Search |
| 4 | ✗ FAIL | 24.4s | 8 | 33074 | 612 | 5344 | $0.0000 | Edit×3, Read×3, Search |
| 5 | ✓ PASS | 21.2s | 8 | 32779 | 531 | 5159 | $0.0000 | Edit×3, Read×3, Search |
| 6 | ✗ FAIL | 8.7s | 5 | 18257 | 196 | 4165 | $0.0000 | Edit, Read×2, Search |
| 7 | ✓ PASS | 23.1s | 8 | 32825 | 593 | 5264 | $0.0000 | Edit×3, Read×3, Search |
| 8 | ✗ FAIL | 24.9s | 8 | 33284 | 616 | 5690 | $0.0000 | Edit×3, Read×3, Search |
| 9 | ✗ FAIL | 24.6s | 8 | 33082 | 607 | 5338 | $0.0000 | Edit×3, Read×3, Search |
| 10 | ✗ FAIL | 14.4s | 7 | 27455 | 335 | 4738 | $0.0000 | Edit×2, Read×3, Search |

**Failures:**

**Run 1**: 
```diff
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -3,5 +3,8 @@
 }
 
 function deprecatedFunc() {}
+}
+
+function deprecatedFunc() {}
 function newFunc() {}
 

```


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
file utils.ts content does not match expected:
--- expected
+++ actual
@@ -2,6 +2,37 @@
     newFunc();
 }
 
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
+
+
+
 function deprecatedFunc() {}
 function newFunc() {}
 

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
@@ -1,5 +1,5 @@
 function helper() {
-    newFunc();
+    deprecatedFunc();
 }
 
 function deprecatedFunc() {}

```


---

#### C5

**✗ Needle in Haystack Search** | Find specific initialization among many usages | 40% (4/10) | 3.3 calls | 11990 tokens | 5.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 5.1s | 2 | 7989 | 106 | 5069 | $0.0000 | Search |
| 2 | ✓ PASS | 5.0s | 2 | 7982 | 99 | 5062 | $0.0000 | Search |
| 3 | ✗ FAIL | 2.7s | 2 | 5949 | 64 | 3015 | $0.0000 | Search |
| 4 | ✗ FAIL | 24.0s | 15 | 58210 | 605 | 5890 | $0.0000 | Read×3, Search×11 |
| 5 | ✗ FAIL | 3.1s | 2 | 5938 | 59 | 3009 | $0.0000 | Search |
| 6 | ✓ PASS | 5.3s | 2 | 7992 | 109 | 5072 | $0.0000 | Search |
| 7 | ✗ FAIL | 3.0s | 2 | 5957 | 78 | 3028 | $0.0000 | Search |
| 8 | ✗ FAIL | 2.9s | 2 | 5957 | 78 | 3028 | $0.0000 | Search |
| 9 | ✓ PASS | 5.2s | 2 | 7993 | 110 | 5073 | $0.0000 | Search |
| 10 | ✗ FAIL | 2.1s | 2 | 5934 | 55 | 3005 | $0.0000 | Search |

**Failures:**

**Run 3**: output does not contain 'wire.go'

**Run 4**: output does not contain 'wire.go'

**Run 5**: output does not contain 'wire.go'

**Run 7**: output does not contain 'wire.go'

**Run 8**: output does not contain 'wire.go'

**Run 10**: output does not contain 'wire.go'

---

#### E1

**✓ Single Line Replace** | Replace a single line | 100% (10/10) | 2.0 calls | 6136 tokens | 3.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.3s | 2 | 6148 | 93 | 3195 | $0.0000 | Edit |
| 2 | ✓ PASS | 3.4s | 2 | 6128 | 73 | 3175 | $0.0000 | Edit |
| 3 | ✓ PASS | 3.7s | 2 | 6152 | 97 | 3199 | $0.0000 | Edit |
| 4 | ✓ PASS | 3.2s | 2 | 6136 | 81 | 3183 | $0.0000 | Edit |
| 5 | ✓ PASS | 2.9s | 2 | 6128 | 73 | 3175 | $0.0000 | Edit |
| 6 | ✓ PASS | 3.2s | 2 | 6138 | 83 | 3185 | $0.0000 | Edit |
| 7 | ✓ PASS | 3.5s | 2 | 6144 | 89 | 3191 | $0.0000 | Edit |
| 8 | ✓ PASS | 3.2s | 2 | 6136 | 81 | 3183 | $0.0000 | Edit |
| 9 | ✓ PASS | 2.8s | 2 | 6124 | 71 | 3173 | $0.0000 | Edit |
| 10 | ✓ PASS | 3.1s | 2 | 6128 | 73 | 3175 | $0.0000 | Edit |

---

#### E2

**✗ Multi-line Insert** | Insert multiple lines at position | 0% (0/10) | 3.0 calls | 9934 tokens | 6.6s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 6.1s | 3 | 9831 | 136 | 3611 | $0.0000 | Edit, Read |
| 2 | ✗ FAIL | 5.9s | 3 | 9949 | 140 | 3703 | $0.0000 | Edit, Read |
| 3 | ✗ FAIL | 6.7s | 3 | 9863 | 168 | 3643 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 6.3s | 3 | 9910 | 143 | 3673 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 6.2s | 3 | 9846 | 137 | 3615 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 5.7s | 3 | 9825 | 125 | 3597 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 6.1s | 3 | 9948 | 139 | 3702 | $0.0000 | Edit, Read |
| 8 | ✗ FAIL | 5.0s | 3 | 9811 | 116 | 3591 | $0.0000 | Edit, Read |
| 9 | ✗ FAIL | 7.3s | 3 | 9890 | 181 | 3659 | $0.0000 | Edit, Read |
| 10 | ✗ FAIL | 11.0s | 3 | 10465 | 239 | 4127 | $0.0000 | Edit, Read |

**Failures:**

**Run 1**: 
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


**Run 2**: 
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


**Run 3**: 
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


**Run 4**: 
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
-FUNCTION K
+   11│INSERTED
+  12│INSERTED
+  13│INSERTED
+  14│FUNCTION K
 FUNCTION L
 FUNCTION M
 FUNCTION N

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
+        INSERTED
+        INSERTED
+        INSERTED
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


**Run 8**: 
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
+        INSERTED
+        INSERTED
+        INSERTED
 FUNCTION K
 FUNCTION L
 FUNCTION M

```


**Run 10**: 
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


---

#### E3

**✗ Delete Lines** | Delete a range of lines | 0% (0/10) | 3.0 calls | 9776 tokens | 5.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 4.7s | 3 | 9711 | 105 | 3516 | $0.0000 | Edit, Read |
| 2 | ✗ FAIL | 4.4s | 3 | 9711 | 105 | 3516 | $0.0000 | Edit, Read |
| 3 | ✗ FAIL | 7.1s | 3 | 10129 | 170 | 3880 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 5.9s | 3 | 9875 | 140 | 3653 | $0.0000 | Edit, Read |
| 5 | ✗ FAIL | 4.9s | 3 | 9718 | 112 | 3523 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 5.4s | 3 | 9729 | 123 | 3534 | $0.0000 | Edit, Read |
| 7 | ✗ FAIL | 5.1s | 3 | 9727 | 121 | 3532 | $0.0000 | Edit, Read |
| 8 | ✗ FAIL | 4.7s | 3 | 9718 | 112 | 3523 | $0.0000 | Edit, Read |
| 9 | ✗ FAIL | 4.8s | 3 | 9725 | 119 | 3530 | $0.0000 | Edit, Read |
| 10 | ✗ FAIL | 4.5s | 3 | 9717 | 111 | 3522 | $0.0000 | Edit, Read |

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
@@ -8,6 +8,24 @@
 function H
 function I
 function J
+function A
+function B
+function C
+function D
+function E
+function F
+function G
+function H
+function I
+function J
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

**✗ Boundary Edit** | Test boundary conditions | 0% (0/10) | 2.0 calls | 6078 tokens | 3.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 3.2s | 2 | 6074 | 77 | 3127 | $0.0000 | Edit |
| 2 | ✗ FAIL | 3.1s | 2 | 6074 | 77 | 3127 | $0.0000 | Edit |
| 3 | ✗ FAIL | 3.2s | 2 | 6073 | 76 | 3126 | $0.0000 | Edit |
| 4 | ✗ FAIL | 3.3s | 2 | 6074 | 77 | 3127 | $0.0000 | Edit |
| 5 | ✗ FAIL | 3.7s | 2 | 6087 | 90 | 3140 | $0.0000 | Edit |
| 6 | ✗ FAIL | 2.8s | 2 | 6068 | 71 | 3121 | $0.0000 | Edit |
| 7 | ✗ FAIL | 3.6s | 2 | 6090 | 93 | 3143 | $0.0000 | Edit |
| 8 | ✗ FAIL | 3.4s | 2 | 6085 | 88 | 3138 | $0.0000 | Edit |
| 9 | ✗ FAIL | 3.4s | 2 | 6086 | 89 | 3139 | $0.0000 | Edit |
| 10 | ✗ FAIL | 3.0s | 2 | 6073 | 76 | 3126 | $0.0000 | Edit |

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

**✗ Replace All Occurrences** | Replace multiple occurrences | 50% (5/10) | 4.0 calls | 14377 tokens | 9.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 11.8s | 4 | 14779 | 251 | 4422 | $0.0000 | Edit, Read, Search |
| 2 | ✗ FAIL | 3.9s | 2 | 6192 | 90 | 3233 | $0.0000 | Edit |
| 3 | ✗ FAIL | 7.4s | 4 | 12003 | 162 | 3185 | $0.0000 | Edit |
| 4 | ✗ FAIL | 14.2s | 7 | 28335 | 337 | 4989 | $0.0000 | Edit×4, Read, Search |
| 5 | ✗ FAIL | 3.5s | 2 | 6121 | 82 | 3175 | $0.0000 | Edit |
| 6 | ✗ FAIL | 3.8s | 2 | 6131 | 92 | 3185 | $0.0000 | Edit |
| 7 | ✓ PASS | 11.1s | 6 | 23299 | 256 | 4634 | $0.0000 | Edit×4, Search |
| 8 | ✓ PASS | 9.7s | 4 | 14545 | 217 | 4290 | $0.0000 | Edit, Read, Search |
| 9 | ✓ PASS | 13.2s | 4 | 14634 | 303 | 4376 | $0.0000 | Edit, Read, Search |
| 10 | ✓ PASS | 13.4s | 5 | 17735 | 306 | 4442 | $0.0000 | Edit, Read, Search |

**Failures:**

**Run 2**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -1,12 +1,11 @@
-function main() {
-    const result = newFunc();
-    if (newFunc() !== null) {
-        return;
+function newFunc() {
+  console.log('newFunc');
+}
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


**Run 4**: 
```diff
file code.ts content does not match expected:
--- expected
+++ actual
@@ -3,10 +3,11 @@
     if (newFunc() !== null) {
         return;
     }
+    }
     const value = newFunc();
 }
-
 function newFunc(): string | null {
     return null;
 }
+}
 

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


**Run 6**: 
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


---

#### E6

**✗ Context-Specific Replace** | Replace with context for uniqueness | 30% (3/10) | 2.9 calls | 9037 tokens | 4.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 5.7s | 3 | 9613 | 136 | 3493 | $0.0000 | Edit, Search |
| 2 | ✗ FAIL | 3.9s | 2 | 6045 | 96 | 3100 | $0.0000 | Edit |
| 3 | ✗ FAIL | 3.6s | 3 | 9032 | 81 | 3102 | $0.0000 | Read, Write |
| 4 | ✗ FAIL | 2.8s | 2 | 6012 | 63 | 3067 | $0.0000 | Edit |
| 5 | ✓ PASS | 5.2s | 3 | 9591 | 114 | 3471 | $0.0000 | Edit, Search |
| 6 | ✗ FAIL | 7.2s | 5 | 16064 | 158 | 3616 | $0.0000 | Edit, Read×3 |
| 7 | ✗ FAIL | 2.7s | 2 | 6047 | 68 | 3102 | $0.0000 | Edit |
| 8 | ✗ FAIL | 2.7s | 2 | 5947 | 68 | 3029 | $0.0000 | Read |
| 9 | ✓ PASS | 6.6s | 5 | 16008 | 156 | 3582 | $0.0000 | Edit, Read×3 |
| 10 | ✗ FAIL | 2.9s | 2 | 6015 | 66 | 3070 | $0.0000 | Edit |

**Failures:**

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
@@ -1,4 +1,6 @@
 const user = { value: 1 };
+
 const config = { value: 99 };
+
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

**✗ Multi-line Block Replace** | Replace multi-line block | 0% (0/10) | 2.4 calls | 7436 tokens | 3.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 3.1s | 2 | 6037 | 74 | 3086 | $0.0000 | Edit |
| 2 | ✗ FAIL | 3.1s | 2 | 6064 | 71 | 3113 | $0.0000 | Edit |
| 3 | ✗ FAIL | 3.1s | 2 | 6064 | 71 | 3113 | $0.0000 | Edit |
| 4 | ✗ FAIL | 2.9s | 2 | 6064 | 71 | 3113 | $0.0000 | Edit |
| 5 | ✗ FAIL | 3.6s | 2 | 6052 | 89 | 3101 | $0.0000 | Edit |
| 6 | ✗ FAIL | 3.1s | 2 | 6070 | 73 | 3117 | $0.0000 | Edit |
| 7 | ✗ FAIL | 3.1s | 2 | 6070 | 73 | 3117 | $0.0000 | Edit |
| 8 | ✗ FAIL | 3.0s | 2 | 6064 | 71 | 3113 | $0.0000 | Edit |
| 9 | ✗ FAIL | 11.4s | 6 | 19804 | 277 | 3994 | $0.0000 | Edit, Read×4 |
| 10 | ✗ FAIL | 3.1s | 2 | 6070 | 73 | 3117 | $0.0000 | Edit |

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
@@ -6,3 +6,7 @@
     return 0;
 }
 
+function other(): number {
+    return 0;
+}
+

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

**✗ No Match Handling** | Handle search text not found | 0% (0/10) | 2.1 calls | 6383 tokens | 3.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 3.1s | 2 | 6056 | 75 | 3121 | $0.0000 | Edit |
| 2 | ✗ FAIL | 3.2s | 2 | 6059 | 78 | 3124 | $0.0000 | Edit |
| 3 | ✗ FAIL | 2.9s | 2 | 6051 | 71 | 3108 | $0.0000 | Edit |
| 4 | ✗ FAIL | 3.1s | 2 | 6056 | 76 | 3113 | $0.0000 | Edit |
| 5 | ✗ FAIL | 2.5s | 2 | 6043 | 62 | 3108 | $0.0000 | Edit |
| 6 | ✗ FAIL | 3.3s | 2 | 6062 | 82 | 3119 | $0.0000 | Edit |
| 7 | ✗ FAIL | 5.0s | 3 | 9323 | 119 | 3273 | $0.0000 | Edit, Read |
| 8 | ✗ FAIL | 3.1s | 2 | 6056 | 76 | 3113 | $0.0000 | Edit |
| 9 | ✗ FAIL | 3.0s | 2 | 6055 | 75 | 3112 | $0.0000 | Edit |
| 10 | ✗ FAIL | 3.1s | 2 | 6072 | 75 | 3135 | $0.0000 | Edit |

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

output contains 'replaced' but should not
output contains 'successfully' but should not

**Run 3**: 
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

**Run 7**: output contains 'replaced' but should not
output contains 'successfully' but should not

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

output contains 'replaced' but should not
output contains 'successfully' but should not

**Run 10**: 
```diff
file sample.txt content does not match expected:
--- expected
+++ actual
@@ -1,3 +1,4 @@
+targetString -> replacement
 This is a sample file.
 It has multiple lines.
 Nothing special here.

```

output contains 'replaced' but should not
output contains 'successfully' but should not

---

#### E9

**✓ Empty Content** | Handle empty replacement | 100% (10/10) | 3.0 calls | 9548 tokens | 4.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.7s | 3 | 9541 | 107 | 3422 | $0.0000 | Edit, Read |
| 2 | ✓ PASS | 5.1s | 3 | 9555 | 121 | 3436 | $0.0000 | Edit, Read |
| 3 | ✓ PASS | 4.6s | 3 | 9542 | 108 | 3423 | $0.0000 | Edit, Read |
| 4 | ✓ PASS | 6.4s | 3 | 9594 | 160 | 3475 | $0.0000 | Edit, Read |
| 5 | ✓ PASS | 4.9s | 3 | 9549 | 115 | 3430 | $0.0000 | Edit, Read |
| 6 | ✓ PASS | 5.1s | 3 | 9555 | 121 | 3436 | $0.0000 | Edit, Read |
| 7 | ✓ PASS | 4.4s | 3 | 9536 | 102 | 3417 | $0.0000 | Edit, Read |
| 8 | ✓ PASS | 4.2s | 3 | 9531 | 97 | 3412 | $0.0000 | Edit, Read |
| 9 | ✓ PASS | 4.2s | 3 | 9532 | 98 | 3413 | $0.0000 | Edit, Read |
| 10 | ✓ PASS | 4.6s | 3 | 9543 | 109 | 3424 | $0.0000 | Edit, Read |

---

#### E10

**✗ Special Characters** | Handle special characters in replacement | 0% (0/10) | 2.2 calls | 6691 tokens | 4.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 3.5s | 2 | 6095 | 85 | 3144 | $0.0000 | Edit |
| 2 | ✗ FAIL | 3.5s | 2 | 6094 | 84 | 3143 | $0.0000 | Edit |
| 3 | ✗ FAIL | 3.8s | 2 | 6106 | 96 | 3155 | $0.0000 | Edit |
| 4 | ✗ FAIL | 3.8s | 2 | 6118 | 94 | 3159 | $0.0000 | Edit |
| 5 | ✗ FAIL | 3.4s | 2 | 6095 | 85 | 3144 | $0.0000 | Edit |
| 6 | ✗ FAIL | 5.6s | 3 | 9053 | 138 | 3154 | $0.0000 | Edit |
| 7 | ✗ FAIL | 3.4s | 2 | 6094 | 84 | 3143 | $0.0000 | Edit |
| 8 | ✗ FAIL | 4.0s | 2 | 6124 | 100 | 3165 | $0.0000 | Edit |
| 9 | ✗ FAIL | 5.2s | 3 | 9042 | 127 | 3143 | $0.0000 | Edit |
| 10 | ✗ FAIL | 3.3s | 2 | 6092 | 82 | 3141 | $0.0000 | Edit |

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

**✗ Indentation Preservation** | Maintain correct indentation (critical for Python) | 10% (1/10) | 2.5 calls | 7810 tokens | 4.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 4.1s | 2 | 6200 | 102 | 3233 | $0.0000 | Edit |
| 2 | ✗ FAIL | 4.1s | 2 | 6202 | 102 | 3235 | $0.0000 | Edit |
| 3 | ✗ FAIL | 5.5s | 3 | 9578 | 131 | 3460 | $0.0000 | Edit, Read |
| 4 | ✗ FAIL | 3.7s | 2 | 6188 | 90 | 3221 | $0.0000 | Edit |
| 5 | ✗ FAIL | 7.2s | 4 | 12531 | 175 | 3456 | $0.0000 | Edit, Read |
| 6 | ✗ FAIL | 4.4s | 2 | 6211 | 110 | 3244 | $0.0000 | Edit |
| 7 | ✗ FAIL | 3.9s | 2 | 6196 | 94 | 3229 | $0.0000 | Edit |
| 8 | ✗ FAIL | 4.2s | 2 | 6204 | 103 | 3237 | $0.0000 | Edit |
| 9 | ✗ FAIL | 5.9s | 3 | 9157 | 144 | 3221 | $0.0000 | Edit |
| 10 | ✓ PASS | 5.9s | 3 | 9636 | 137 | 3496 | $0.0000 | Edit, Read |

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
-        finalize()
+        # placeholder
+validate_input()
+transform_data()
 

```


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
-        finalize()
+        # placeholder
+validate_input()
+transform_data()
 

```


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
+validate_input()
+transform_data()
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
+        # placeholder
+validate_input()
+transform_data()
 

```


---

#### R1

**✗ Simple File Read** | Read an entire small file | 90% (9/10) | 2.0 calls | 6032 tokens | 2.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.4s | 2 | 6024 | 60 | 3112 | $0.0000 | Read |
| 2 | ✓ PASS | 1.4s | 2 | 6024 | 60 | 3112 | $0.0000 | Read |
| 3 | ✓ PASS | 1.3s | 2 | 6021 | 57 | 3109 | $0.0000 | Read |
| 4 | ✓ PASS | 1.6s | 2 | 6035 | 71 | 3123 | $0.0000 | Read |
| 5 | ✗ FAIL | 2.8s | 2 | 6017 | 53 | 3105 | $0.0000 | Read |
| 6 | ✓ PASS | 2.4s | 2 | 6024 | 60 | 3112 | $0.0000 | Read |
| 7 | ✓ PASS | 2.8s | 2 | 6038 | 74 | 3126 | $0.0000 | Read |
| 8 | ✓ PASS | 3.2s | 2 | 6051 | 87 | 3139 | $0.0000 | Read |
| 9 | ✓ PASS | 2.3s | 2 | 6022 | 58 | 3110 | $0.0000 | Read |
| 10 | ✓ PASS | 3.6s | 2 | 6061 | 97 | 3149 | $0.0000 | Read |

**Failures:**

**Run 5**: output does not contain 'db_host=localhost'

---

#### R2

**✓ Truncation Recovery** | Handle truncated output by chunked reading | 100% (10/10) | 2.1 calls | 7100 tokens | 4.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 4.7s | 2 | 6723 | 112 | 3803 | $0.0000 | Read |
| 2 | ✓ PASS | 5.2s | 2 | 6736 | 125 | 3816 | $0.0000 | Read |
| 3 | ✓ PASS | 4.1s | 2 | 6714 | 103 | 3794 | $0.0000 | Read |
| 4 | ✓ PASS | 3.5s | 2 | 6694 | 83 | 3774 | $0.0000 | Read |
| 5 | ✓ PASS | 5.2s | 2 | 6737 | 126 | 3817 | $0.0000 | Read |
| 6 | ✓ PASS | 3.6s | 2 | 6694 | 83 | 3774 | $0.0000 | Read |
| 7 | ✓ PASS | 4.9s | 2 | 6731 | 120 | 3811 | $0.0000 | Read |
| 8 | ✓ PASS | 3.5s | 2 | 6691 | 80 | 3771 | $0.0000 | Read |
| 9 | ✓ PASS | 6.3s | 3 | 10601 | 151 | 3922 | $0.0000 | Read, Shell |
| 10 | ✓ PASS | 3.2s | 2 | 6684 | 73 | 3764 | $0.0000 | Read |

---

#### R3

**✓ Relative vs Absolute Path** | Correct path resolution | 100% (10/10) | 2.0 calls | 5997 tokens | 2.0s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.0s | 2 | 5994 | 47 | 3076 | $0.0000 | Read |
| 2 | ✓ PASS | 1.9s | 2 | 5995 | 47 | 3077 | $0.0000 | Read |
| 3 | ✓ PASS | 2.0s | 2 | 5998 | 50 | 3080 | $0.0000 | Read |
| 4 | ✓ PASS | 1.7s | 2 | 5992 | 44 | 3074 | $0.0000 | Read |
| 5 | ✓ PASS | 1.8s | 2 | 5994 | 46 | 3076 | $0.0000 | Read |
| 6 | ✓ PASS | 2.3s | 2 | 6007 | 60 | 3089 | $0.0000 | Read |
| 7 | ✓ PASS | 1.9s | 2 | 5994 | 47 | 3076 | $0.0000 | Read |
| 8 | ✓ PASS | 2.5s | 2 | 6012 | 65 | 3094 | $0.0000 | Read |
| 9 | ✓ PASS | 1.9s | 2 | 5994 | 47 | 3076 | $0.0000 | Read |
| 10 | ✓ PASS | 1.9s | 2 | 5994 | 46 | 3076 | $0.0000 | Read |

---

#### S1

**✗ Simple Pattern Search** | Find a specific function definition | 10% (1/10) | 3.4 calls | 11114 tokens | 4.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 4.9s | 4 | 13324 | 184 | 3787 | $0.0000 | Read×2, Search |
| 2 | ✗ FAIL | 1.3s | 2 | 5928 | 55 | 3008 | $0.0000 | Search |
| 3 | ✓ PASS | 7.1s | 4 | 13392 | 249 | 3850 | $0.0000 | Read×2, Search |
| 4 | ✗ FAIL | 1.3s | 2 | 5930 | 57 | 3010 | $0.0000 | Search |
| 5 | ✗ FAIL | 6.4s | 4 | 13412 | 269 | 3870 | $0.0000 | Read×2, Search |
| 6 | ✗ FAIL | 4.6s | 4 | 13224 | 177 | 3742 | $0.0000 | Read×2, Search |
| 7 | ✗ FAIL | 4.9s | 4 | 13333 | 190 | 3791 | $0.0000 | Read×2, Search |
| 8 | ✗ FAIL | 1.2s | 2 | 5924 | 51 | 3004 | $0.0000 | Search |
| 9 | ✗ FAIL | 5.0s | 4 | 13338 | 195 | 3796 | $0.0000 | Read×2, Search |
| 10 | ✗ FAIL | 5.0s | 4 | 13338 | 195 | 3796 | $0.0000 | Read×2, Search |

**Failures:**

**Run 1**: output does not contain 'func calculateTotal(items []int)'

**Run 2**: output does not contain 'func calculateTotal(items []int)'

**Run 4**: output does not contain 'func calculateTotal(items []int)'

**Run 5**: output does not contain 'func calculateTotal(items []int)'

**Run 6**: output does not contain 'func calculateTotal(items []int)'

**Run 7**: output does not contain 'func calculateTotal(items []int)'

**Run 8**: output does not contain 'func calculateTotal(items []int)'

**Run 9**: output does not contain 'func calculateTotal(items []int)'

**Run 10**: output does not contain 'func calculateTotal(items []int)'

---

#### S2

**✗ Multi-Pattern Search** | Find multiple related items | 40% (4/10) | 1.8 calls | 5448 tokens | 14.9s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.7s | 2 | 5911 | 48 | 2983 | $0.0000 | Shell |
| 2 | ✓ PASS | 4.5s | 2 | 6150 | 166 | 3101 | $0.0000 | Shell |
| 3 | ✓ PASS | 7.1s | 2 | 6342 | 262 | 3197 | $0.0000 | Shell |
| 4 | ✓ PASS | 5.1s | 2 | 6196 | 189 | 3124 | $0.0000 | Shell |
| 5 | ✗ FAIL | 120.0s | 0 | 0 | 0 | 0 | $0.0000 | - |
| 6 | ✗ FAIL | 1.9s | 2 | 5944 | 63 | 2998 | $0.0000 | Shell |
| 7 | ✗ FAIL | 1.5s | 2 | 5927 | 64 | 2999 | $0.0000 | Shell |
| 8 | ✓ PASS | 3.5s | 2 | 6080 | 132 | 3067 | $0.0000 | Shell |
| 9 | ✗ FAIL | 2.1s | 2 | 5980 | 81 | 3016 | $0.0000 | Shell |
| 10 | ✗ FAIL | 1.7s | 2 | 5946 | 65 | 3000 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain '7'

**Run 5**: output does not contain '7'

**Run 6**: output does not contain '7'

**Run 7**: output does not contain '7'

**Run 9**: output does not contain '7'

**Run 10**: output does not contain '7'

---

#### S3

**✗ Search with File Filtering** | Search in specific file types | 10% (1/10) | 2.0 calls | 5993 tokens | 1.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 1.2s | 2 | 5955 | 44 | 3005 | $0.0000 | Search |
| 2 | ✗ FAIL | 1.0s | 2 | 5942 | 37 | 2999 | $0.0000 | Search |
| 3 | ✗ FAIL | 1.2s | 2 | 5951 | 46 | 3008 | $0.0000 | Search |
| 4 | ✓ PASS | 1.5s | 2 | 5970 | 55 | 3009 | $0.0000 | Shell |
| 5 | ✗ FAIL | 1.2s | 2 | 5951 | 46 | 3008 | $0.0000 | Search |
| 6 | ✗ FAIL | 1.0s | 2 | 5942 | 37 | 2999 | $0.0000 | Search |
| 7 | ✗ FAIL | 2.4s | 2 | 6036 | 88 | 3046 | $0.0000 | Shell |
| 8 | ✗ FAIL | 1.1s | 2 | 5949 | 40 | 3003 | $0.0000 | Search |
| 9 | ✗ FAIL | 1.2s | 2 | 5951 | 46 | 3008 | $0.0000 | Search |
| 10 | ✗ FAIL | 1.1s | 2 | 6279 | 36 | 3341 | $0.0000 | Search |

**Failures:**

**Run 1**: output does not contain 'models.go'

**Run 2**: output does not contain 'models.go'

**Run 3**: output does not contain 'models.go'

**Run 5**: output does not contain 'models.go'

**Run 6**: output does not contain 'models.go'

**Run 7**: output contains 'helpers.go' but should not

**Run 8**: output does not contain 'models.go'

**Run 9**: output does not contain 'models.go'

**Run 10**: output contains 'helpers.go' but should not

---

#### S4

**✗ Search Truncation Recovery** | Handle large search results requiring refinement | 10% (1/10) | 3.3 calls | 23418 tokens | 7.4s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✗ FAIL | 41.8s | 9 | 150153 | 1332 | 22280 | $0.0000 | Read×3, Search, Shell.advanced×2, W... |
| 2 | ✗ FAIL | 6.2s | 3 | 8989 | 189 | 3036 | $0.0000 | Shell |
| 3 | ✓ PASS | 7.9s | 5 | 23693 | 291 | 5465 | $0.0000 | Read×2, Search, Shell |
| 4 | ✗ FAIL | 2.7s | 2 | 6002 | 113 | 3046 | $0.0000 | Shell |
| 5 | ✗ FAIL | 2.1s | 2 | 5981 | 84 | 3030 | $0.0000 | Search |
| 6 | ✗ FAIL | 1.6s | 2 | 5931 | 66 | 3003 | $0.0000 | Shell |
| 7 | ✗ FAIL | 1.8s | 2 | 7791 | 44 | 4877 | $0.0000 | Search |
| 8 | ✗ FAIL | 1.7s | 2 | 7784 | 37 | 4870 | $0.0000 | Search |
| 9 | ✗ FAIL | 1.8s | 2 | 5941 | 76 | 3013 | $0.0000 | Shell |
| 10 | ✗ FAIL | 6.7s | 4 | 11913 | 258 | 3008 | $0.0000 | Shell |

**Failures:**

**Run 1**: output does not contain 'folder3'

**Run 2**: output does not contain 'folder3'

**Run 4**: output does not contain 'folder3'

**Run 5**: output does not contain 'folder3'

**Run 6**: output does not contain 'folder3'

**Run 7**: output does not contain 'folder3'

**Run 8**: output does not contain 'folder3'

**Run 9**: output does not contain 'folder3'

**Run 10**: output does not contain 'folder3'

---

#### W1

**✓ Simple File Creation** | Create a new file with content | 100% (10/10) | 2.0 calls | 5939 tokens | 2.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.0s | 2 | 5933 | 49 | 3007 | $0.0000 | Write |
| 2 | ✓ PASS | 2.4s | 2 | 5945 | 61 | 3019 | $0.0000 | Write |
| 3 | ✓ PASS | 2.4s | 2 | 5945 | 61 | 3019 | $0.0000 | Write |
| 4 | ✓ PASS | 2.1s | 2 | 5934 | 50 | 3008 | $0.0000 | Write |
| 5 | ✓ PASS | 2.2s | 2 | 5941 | 57 | 3015 | $0.0000 | Write |
| 6 | ✓ PASS | 2.4s | 2 | 5945 | 61 | 3019 | $0.0000 | Write |
| 7 | ✓ PASS | 2.4s | 2 | 5945 | 61 | 3019 | $0.0000 | Write |
| 8 | ✓ PASS | 2.1s | 2 | 5934 | 50 | 3008 | $0.0000 | Write |
| 9 | ✓ PASS | 1.8s | 2 | 5933 | 49 | 3007 | $0.0000 | Write |
| 10 | ✓ PASS | 1.9s | 2 | 5934 | 50 | 3008 | $0.0000 | Write |

---

#### W2

**✓ Multi-line Content** | Write file with multiple lines and formatting | 100% (10/10) | 2.0 calls | 6000 tokens | 3.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 3.4s | 2 | 6004 | 89 | 3051 | $0.0000 | Write |
| 2 | ✓ PASS | 3.1s | 2 | 5992 | 77 | 3039 | $0.0000 | Write |
| 3 | ✓ PASS | 3.1s | 2 | 5990 | 75 | 3037 | $0.0000 | Write |
| 4 | ✓ PASS | 4.2s | 2 | 6022 | 107 | 3069 | $0.0000 | Write |
| 5 | ✓ PASS | 3.1s | 2 | 5989 | 74 | 3036 | $0.0000 | Write |
| 6 | ✓ PASS | 3.6s | 2 | 6004 | 89 | 3051 | $0.0000 | Write |
| 7 | ✓ PASS | 3.5s | 2 | 6005 | 90 | 3052 | $0.0000 | Write |
| 8 | ✓ PASS | 3.0s | 2 | 5992 | 77 | 3039 | $0.0000 | Write |
| 9 | ✓ PASS | 3.5s | 2 | 6006 | 91 | 3053 | $0.0000 | Write |
| 10 | ✓ PASS | 3.0s | 2 | 5992 | 77 | 3039 | $0.0000 | Write |

---

#### W3

**✗ Overwrite Existing** | Handle overwrite of existing file | 90% (9/10) | 2.9 calls | 8802 tokens | 2.8s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.7s | 3 | 9110 | 61 | 3133 | $0.0000 | Write, Write.confirm |
| 2 | ✓ PASS | 2.7s | 3 | 9112 | 63 | 3135 | $0.0000 | Write, Write.confirm |
| 3 | ✓ PASS | 3.8s | 3 | 9115 | 66 | 3138 | $0.0000 | Write, Write.confirm |
| 4 | ✓ PASS | 2.9s | 3 | 9112 | 63 | 3135 | $0.0000 | Write, Write.confirm |
| 5 | ✓ PASS | 2.8s | 3 | 9110 | 61 | 3133 | $0.0000 | Write, Write.confirm |
| 6 | ✓ PASS | 3.1s | 3 | 9121 | 72 | 3144 | $0.0000 | Write, Write.confirm |
| 7 | ✓ PASS | 2.7s | 3 | 9115 | 66 | 3138 | $0.0000 | Write, Write.confirm |
| 8 | ✓ PASS | 2.6s | 3 | 9110 | 61 | 3133 | $0.0000 | Write, Write.confirm |
| 9 | ✓ PASS | 2.6s | 3 | 9109 | 60 | 3132 | $0.0000 | Write, Write.confirm |
| 10 | ✗ FAIL | 2.7s | 2 | 6005 | 74 | 3081 | $0.0000 | Write |

**Failures:**

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

**✗ Special Characters** | Handle special characters in content | 90% (9/10) | 2.0 calls | 5970 tokens | 2.3s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.2s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 2 | ✓ PASS | 2.1s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 3 | ✓ PASS | 2.2s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 4 | ✓ PASS | 2.2s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 5 | ✓ PASS | 2.2s | 2 | 5965 | 52 | 3021 | $0.0000 | Write |
| 6 | ✓ PASS | 2.2s | 2 | 5966 | 53 | 3022 | $0.0000 | Write |
| 7 | ✗ FAIL | 2.4s | 2 | 5967 | 54 | 3022 | $0.0000 | Write |
| 8 | ✓ PASS | 2.2s | 2 | 5965 | 52 | 3021 | $0.0000 | Write |
| 9 | ✓ PASS | 2.1s | 2 | 5965 | 52 | 3021 | $0.0000 | Write |
| 10 | ✓ PASS | 3.5s | 2 | 6009 | 96 | 3065 | $0.0000 | Write |

**Failures:**

**Run 7**: 
```diff
file special.txt content does not match expected:
--- expected
+++ actual
@@ -1,2 +1 @@
-'quotes', "double", `backticks`, $var
-
+"quotes", "double", `backticks`, $var

```


---

#### W5

**✓ Empty File Creation** | Create an empty file | 100% (10/10) | 2.0 calls | 5912 tokens | 1.7s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 1.5s | 2 | 5907 | 37 | 2990 | $0.0000 | Write |
| 2 | ✓ PASS | 1.7s | 2 | 5914 | 44 | 2997 | $0.0000 | Write |
| 3 | ✓ PASS | 1.9s | 2 | 5919 | 49 | 3002 | $0.0000 | Write |
| 4 | ✓ PASS | 1.5s | 2 | 5910 | 40 | 2993 | $0.0000 | Write |
| 5 | ✓ PASS | 1.5s | 2 | 5906 | 36 | 2989 | $0.0000 | Write |
| 6 | ✓ PASS | 1.5s | 2 | 5907 | 37 | 2990 | $0.0000 | Write |
| 7 | ✓ PASS | 1.9s | 2 | 5919 | 49 | 3002 | $0.0000 | Write |
| 8 | ✓ PASS | 1.7s | 2 | 5915 | 45 | 2998 | $0.0000 | Write |
| 9 | ✓ PASS | 1.5s | 2 | 5907 | 37 | 2990 | $0.0000 | Write |
| 10 | ✓ PASS | 1.9s | 2 | 5917 | 47 | 3000 | $0.0000 | Write |

---

#### W6

**✓ Path with Spaces** | Handle paths with spaces | 100% (10/10) | 2.0 calls | 5950 tokens | 2.2s

| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |
|-----|--------|----------|-----------|--------|-----------|---------|------|-------|
| 1 | ✓ PASS | 2.2s | 2 | 5951 | 52 | 3018 | $0.0000 | Write |
| 2 | ✓ PASS | 2.1s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 3 | ✓ PASS | 2.1s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 4 | ✓ PASS | 2.3s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 5 | ✓ PASS | 2.1s | 2 | 5946 | 47 | 3013 | $0.0000 | Write |
| 6 | ✓ PASS | 2.1s | 2 | 5951 | 52 | 3018 | $0.0000 | Write |
| 7 | ✓ PASS | 2.2s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 8 | ✓ PASS | 2.2s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 9 | ✓ PASS | 2.2s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |
| 10 | ✓ PASS | 2.2s | 2 | 5950 | 51 | 3017 | $0.0000 | Write |

---

## Failure Analysis

| Benchmark | Run | Errors | Last Tool Call |
|-----------|-----|--------|----------------|
| S1 | 1 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 2 | output does not contain 'func calculateTotal(it... | Search |
| S1 | 4 | output does not contain 'func calculateTotal(it... | Search |
| S1 | 5 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 6 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 7 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 8 | output does not contain 'func calculateTotal(it... | Search |
| S1 | 9 | output does not contain 'func calculateTotal(it... | Read |
| S1 | 10 | output does not contain 'func calculateTotal(it... | Read |
| S2 | 1 | output does not contain '7' | Shell |
| S2 | 5 | output does not contain '7' |  |
| S2 | 6 | output does not contain '7' | Shell |
| S2 | 7 | output does not contain '7' | Shell |
| S2 | 9 | output does not contain '7' | Shell |
| S2 | 10 | output does not contain '7' | Shell |
| S3 | 1 | output does not contain 'models.go' | Search |
| S3 | 2 | output does not contain 'models.go' | Search |
| S3 | 3 | output does not contain 'models.go' | Search |
| S3 | 5 | output does not contain 'models.go' | Search |
| S3 | 6 | output does not contain 'models.go' | Search |
| S3 | 7 | output contains 'helpers.go' but should not | Shell |
| S3 | 8 | output does not contain 'models.go' | Search |
| S3 | 9 | output does not contain 'models.go' | Search |
| S3 | 10 | output contains 'helpers.go' but should not | Search |
| S4 | 1 | output does not contain 'folder3' | Write |
| S4 | 2 | output does not contain 'folder3' | Shell |
| S4 | 4 | output does not contain 'folder3' | Shell |
| S4 | 5 | output does not contain 'folder3' | Search |
| S4 | 6 | output does not contain 'folder3' | Shell |
| S4 | 7 | output does not contain 'folder3' | Search |
| S4 | 8 | output does not contain 'folder3' | Search |
| S4 | 9 | output does not contain 'folder3' | Shell |
| S4 | 10 | output does not contain 'folder3' | Shell |
| R1 | 5 | output does not contain 'db_host=localhost' | Read |
| W3 | 10 | file existing.txt content does not match expect... | Write |
| W4 | 7 | file special.txt content does not match expecte... | Write |
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
| E5 | 4 | file code.ts content does not match expected:
-... | Edit |
| E5 | 5 | file code.ts content does not match expected:
-... | Edit |
| E5 | 6 | file code.ts content does not match expected:
-... | Edit |
| E6 | 2 | file values.ts content does not match expected:... | Edit |
| E6 | 3 | file values.ts content does not match expected:... | Write |
| E6 | 4 | file values.ts content does not match expected:... | Edit |
| E6 | 6 | file values.ts content does not match expected:... | Edit |
| E6 | 7 | file values.ts content does not match expected:... | Edit |
| E6 | 8 | file values.ts content does not match expected:... | Read |
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
| E8 | 6 | file sample.txt content does not match expected... | Edit |
| E8 | 7 | output contains 'replaced' but should not; outp... | Edit |
| E8 | 8 | file sample.txt content does not match expected... | Edit |
| E8 | 9 | file sample.txt content does not match expected... | Edit |
| E8 | 10 | file sample.txt content does not match expected... | Edit |
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
| E11 | 5 | file process.py content does not match expected... | Edit |
| E11 | 6 | file process.py content does not match expected... | Edit |
| E11 | 7 | file process.py content does not match expected... | Edit |
| E11 | 8 | file process.py content does not match expected... | Edit |
| E11 | 9 | file process.py content does not match expected... | Edit |
| C1 | 1 | output does not contain 'db.example.com' | Search |
| C1 | 2 | output does not contain 'db.example.com' | Read |
| C1 | 3 | output does not contain 'db.example.com' | Search |
| C1 | 4 | output does not contain 'db.example.com' | Search |
| C1 | 6 | output does not contain 'db.example.com' | Search |
| C1 | 7 | output does not contain 'db.example.com' | Search |
| C1 | 8 | output does not contain 'db.example.com' | Search |
| C1 | 9 | output does not contain 'db.example.com' | Search |
| C1 | 10 | output does not contain 'db.example.com' | Search |
| C2 | 3 | file config.yaml content does not match expecte... |  |
| C2 | 4 | file config.yaml content does not match expecte... | Edit |
| C2 | 9 | file config.yaml content does not match expecte... | Edit |
| C3 | 1 | file utils.ts content does not match expected:
... | Read |
| C3 | 2 | file utils.ts content does not match expected:
... | Edit |
| C3 | 4 | file utils.ts content does not match expected:
... | Edit |
| C3 | 6 | file handler2.ts content does not match expecte... | Read |
| C3 | 8 | file utils.ts content does not match expected:
... | Edit |
| C3 | 9 | file utils.ts content does not match expected:
... | Edit |
| C3 | 10 | file utils.ts content does not match expected:
... | Edit |
| C5 | 3 | output does not contain 'wire.go' | Search |
| C5 | 4 | output does not contain 'wire.go' | Read |
| C5 | 5 | output does not contain 'wire.go' | Search |
| C5 | 7 | output does not contain 'wire.go' | Search |
| C5 | 8 | output does not contain 'wire.go' | Search |
| C5 | 10 | output does not contain 'wire.go' | Search |

## Appendix A: Configuration

### Version

```
kvit-coder de939e8 (commit 20260101, built 2026-01-02)
```

### config.yaml

```yaml

```

