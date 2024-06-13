firstSampled = 3
lookAhead = 10
numElems = 50
lol = []
for i in range(numElems):
    lol.append(max(1, lookAhead - abs(lookAhead - abs(firstSampled - i))))
lol[firstSampled] = 0
print(lol)