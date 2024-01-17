let store;

export function init(nstore) {
	store = nstore
}

function orderAndDedup(arr) {
  const deduped = new Set(arr)
  return [...deduped].sort()
}

// Ordered intersection!
function intersects(a, b) {
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

function addToParents(child, childNestedSubsets) {
  const parents = store.listSupersets(child)
  parents.forEach(set => {
    const all = orderAndDedup([...store.listNestedSubsets(set), ...childNestedSubsets, child])
    store.setNestedSubsets(set, all)
    addToParents(set, all) // RECURSION!!
  })
}

export function addSubset(set, subset) {
  store.addSubset(set, subset)

  const all = orderAndDedup([
		...store.listNestedSubsets(set),
		...store.listNestedSubsets(subset),
		subset,
	])

  store.setNestedSubsets(set, all)
  addToParents(set, all)
}

function computeNestedSubsets(set, normalize = true) {
	const nestedSubsets = store.listSubsets(set)
		.map(set => computeNestedSubsets(set, false))
		.reduce((a, b) => [...a, ...b], [])

	if (!normalize) return nestedSubsets
	return orderAndDedup(nestedSubsets)
}

export function rebuildSupersets(set) {
	const supersets = store.listSupersets(set)
	supersets.forEach(set => {
		const nestedSubsets = computeNestedSubsets(set)
		store.setNestedSubsets(set, nestedSubsets)
		rebuildSupersets(set) // RECURSION
	})
}

export function removeSubset(set, subset) {
  store.removeSubset(set, subset)

  const all = store.listNestedSubsets(set)
		.filter(set => set !== subset)

  store.setNestedSubsets(set, all)
	rebuildSupersets(set)
}

export function check(set, label) {
	if (store.hasLabel(set, label)) return true

	return intersects(
		store.listNestedSubsets(set),
		store.listSetsLabelledWith(label),
	)
}

export function label(set, label) {
	return store.addLabel(set, label)
}

export function unlabel(set, label) {
	return store.removeLabel(set, label)
}
