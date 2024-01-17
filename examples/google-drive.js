import * as z from '../z.js'
import * as store from '../store.js'

store.init()
z.init(store)

// some utils, the format is hard coded

function is(l) {
	return `is:${l}`
}

function u(userId) {
	return `user:${userId}`
}

function resource(type, id) {
	const s = `${type}:${id}`
	return s
}

function checkAnyRelation(res, relations, label) {
	for (let r of relations) {
		const ok = z.check(res.relation(r), label) 
		if (ok) return true
	}
}

String.prototype.relation = function(relation) {
	return this + "#" + relation
}

// implementation

function onCreateFile(folderId, fileId, ownerId) {
	const file = resource("file", fileId)
	const folder = resource("folder", folderId)

	const relations = ["owner", "viewer", "commenter", "editor"]
	for (let relation of relations) {
		z.addSubset(
			file.relation(relation),
			folder.relation(relation),
		)
	}

	z.label(file.relation("owner"), u(ownerId))
}

function shareFolderWithDomain(folderId, domainId) {
	z.addSubset(
		resource("folder", folderId).relation("owner"),
		resource("domain", domainId).relation("member"),
	)
}

function addUserToDomain(domainId, userId) {
	z.label(
		resource("domain", domainId).relation("member"),
		u(userId),
	)
}

function canViewFile(fileId, userId) {
	const isPublic = z.check(resource("file", fileId), is("public"))
	if (isPublic) return true

	return checkAnyRelation(
		resource("file", fileId),
		["owner", "commenter", "editor", "viewer"],
		u(userId),
	)
}

// usage examples

onCreateFile("important-folder", "salaries.pdf", "alice")
console.log(canViewFile("salaries.pdf", "alice"))

addUserToDomain("superadmins", "bob")

shareFolderWithDomain("important-folder", "superadmins")
console.log(canViewFile("salaries.pdf", "bob"))

z.label(resource("file", "salaries.pdf"), is("public"))
console.log(canViewFile("salaries.pdf", "-"))
