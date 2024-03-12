import * as z from '../z.js'
import * as store from '../store.js'

store.init()
z.init(store)

// some utils, the format is hard coded

function is(l) {
    return `is:${l}`
}

function set(namespace, id) {
    const s = `${namespace}:${id}`
    return s
}

function checkAny(namespaces, id, label) {
    for (let n of namespaces) {
	const ok = z.check(set(n, id), label)
	if (ok) return true
    }
    return false
}

// implementation


function onCreateFile(folderId, fileId, ownerId) {
    const relations = ["owner", "viewer", "commenter", "editor"]
    for (let relation of relations) {
	z.addSubset(set(`file_${relation}s`, fileId), set(`folder_${relation}s`, folderId))
    }
    z.label(set("file_owners", fileId), ownerId)
}

function shareFolderWithDomain(folderId, domainId) {
    z.addSubset(set("folder_owners", folderId), set("domain_members", domainId))
}

function addUserToDomain(domainId, userId) {
    z.label(set("domain_members", domainId), userId)
}

function canViewFile(fileId, userId) {
    const isPublic = z.check(set("file_labels", fileId), "public")
    if (isPublic) return true

    return checkAny(
	["file_owners", "file_comenters", "file_editors", "file_viewers"],
	fileId,
	userId,
    )
}

// usage examples

onCreateFile("important-folder", "salaries.pdf", "alice")
console.log(canViewFile("salaries.pdf", "alice"))

addUserToDomain("superadmins", "bob")

shareFolderWithDomain("important-folder", "superadmins")
console.log(canViewFile("salaries.pdf", "bob"))

z.label(set("file_labels", "salaries.pdf"), "public")
console.log(canViewFile("salaries.pdf", "-"))
