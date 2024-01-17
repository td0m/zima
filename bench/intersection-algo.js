function orderAndDedup(arr) {
  const deduped = new Set(arr)
  return [...deduped].sort()
}

// TODO: optimize for ordered
function intersectsNaive(a, b) {
  for(let i = 0; i < a.length; i++) {
    for(let j = 0; j < b.length; j++) {
      if (a[i] === b[j]) return true
    }
  }
  return false
}

// Ordered intersection!
function intersectsOrdered(a, b) {
	let i = 0
	let j = 0
	if (a.length === 0 || b.length === 0) return false
	while (true) {
		let diff = a[i].localeCompare(b[j])
		
		// jth item is after ith
		if (diff < 0) {
			i++
			if (i >= a.length) return false
		} else if (diff > 0) {
			j++
			if (j >= b.length) return false
		} else {
			return true
		}
	}
}

// returns time in ms
function measureTime(repeat, f) {
	let before = process.hrtime.bigint()
	for (let i = 0; i < repeat; i++) {
		f()
	}
	return parseInt((process.hrtime.bigint() - before) / BigInt(1000000)) / repeat
}

// Get a random alpha string of length n
function randomAlphaString(n) {
  let s = "";
  for (let i = 0; i < n; i++) {
    s += String.fromCharCode(Math.floor(Math.random() * 26) + "a".charCodeAt(0));
  }
  return s;
}

function randomArray(n) {
	return orderAndDedup(
		[...Array(n)]
			.map(() => randomAlphaString(50))
	)
}

// sample size
const repeat = 1

// increase the numbers, you'll see the naive time go up and ordered pretty much not at all
// a is a document, its number represents the number of collections of users that can access it
// b is a user, number represent show many resources, collections, etc the given user can access
let a = randomArray(1000)
let b = randomArray(100000)

console.log(
	"naive",
	measureTime(repeat, () => intersectsNaive(a, b)),
)

console.log(
	"ordered",
	measureTime(repeat, () => intersectsOrdered(a, b)),
)
