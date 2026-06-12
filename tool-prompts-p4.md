Assume your ideal file-editing tools. You need to rename the function oldFunc to newFunc at
its one call site in each of three files: handlers/a.py, handlers/b.py, and util/c.py.
Write the EXACT tool call(s) you would make to do all three. Emit exactly what you'd send —
nothing else.

A1:-------------
replace_in_file("handlers/a.py", "oldFunc", "newFunc")
replace_in_file("handlers/b.py", "oldFunc", "newFunc")
replace_in_file("util/c.py", "oldFunc", "newFunc")

A2:-------------
replace_in_file(file="handlers/a.py", old="oldFunc", new="newFunc")
replace_in_file(file="handlers/b.py", old="oldFunc", new="newFunc")
replace_in_file(file="util/c.py", old="oldFunc", new="newFunc")

A3:------------
search_replace(file="handlers/a.py", old="oldFunc", new="newFunc")
search_replace(file="handlers/b.py", old="oldFunc", new="newFunc")
search_replace(file="util/c.py", old="oldFunc", new="newFunc")