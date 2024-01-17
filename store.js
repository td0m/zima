import Database from 'better-sqlite3'

const db = new Database("demo.db")

export function init() {
	db.exec(`
		drop table if exists relations;
		drop table if exists labels;
		drop table if exists set_nested_subsets;

		create table if not exists relations(
			"set"  text not null,
			subset text not null,
			primary key ("set", subset)
		) strict;

		create table if not exists set_nested_subsets(
			"set"  text not null,
			nested_subsets text not null,
			primary key ("set", nested_subsets)
		) strict;

		create table if not exists labels(
			"set"  text not null,
			label  text not null,
			primary key ("set", label)
		) strict;
	`)
}

export function addSubset(set, subset) {
	db.prepare(`
		INSERT INTO relations("set", subset)
		VALUES(:set, :subset)
		ON CONFLICT DO NOTHING
	`).run({set, subset})
}

export function removeSubset(set, subset) {
	db.prepare(`
		DELETE FROM relations
		WHERE ("set", subset) = (:set, :subset)
	`).run({set, subset})
}

export function addLabel(set, label) {
	db.prepare(`
		INSERT INTO labels("set", label)
		VALUES(:set, :label)
		ON CONFLICT DO NOTHING
	`).run({set, label})
}

export function removeLabel(set, label) {
	db.prepare(`
		DELETE FROM labels
		WHERE ("set", label) = (:set, :label)
	`).run({set, label})
}

export function hasLabel(set, label) {
	return db.prepare(`
		SELECT EXISTS(
			SELECT 1
			FROM labels
			WHERE ("set", label) = (:set, :label)
		) "exists"
	`).get({set, label}).exists > 0
}

export function listSubsets(set) {
	try {
		return db.prepare(`
			SELECT subset
			FROM relations
			WHERE "set" = :set
		`).all({set})
			.map(row => row.subset)
	} catch(err) {
		console.log(err)
		return []
	}
}

export function listNestedSubsets(set) {
	try {
		const str = db.prepare(`
			SELECT nested_subsets
			FROM set_nested_subsets
			WHERE "set" = :set
		`).get({set})?.nested_subsets ?? ""
		if (str.length === 0) return []
		return str.split(",")
	} catch(err) {
		console.log(err)
		return []
	}
}

export function setNestedSubsets(set, nested_subsets) {
	return db.prepare(`
		INSERT INTO set_nested_subsets("set", nested_subsets)
		VALUES(:set, :nested_subsets)
		ON CONFLICT DO UPDATE
		SET nested_subsets = :nested_subsets
	`).run({set, nested_subsets: nested_subsets.join(",")})
}

export function listSupersets(subset) {
	try {
		return db.prepare(`
			SELECT "set"
			FROM relations
			WHERE subset = :subset
		`).all({subset})
			.map(row => row.set)
	} catch(err) {
		console.log(err)
		return []
	}
}


export function listSetsLabelledWith(label) {
	try {
		return db.prepare(`
			SELECT "set"
			FROM labels
			WHERE label = :label
		`).all({label})
			.map(row => row.set)
	} catch(err) {
		console.log(err)
		return []
	}
}

