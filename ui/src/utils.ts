import { either, isEmpty, isNil } from "ramda"

export const isNilOrEmpty = either(isNil, isEmpty)

const duplicatedSlashesRegexp = new RegExp("(^\\/|[^:\\/]+\\/)\\/+", "g")

// Given some path segments returns a properly formatted path similarly to Nodejs path.join()
// Remove duplicated slashes
// Does not remove leading/trailing slashes and adds a slash between segments
export const pathJoin = (...pathParts) =>
  []
    .concat(...pathParts) // Flatten
    .filter((segment) => !!segment) // Remove empty parts
    .join("/")
    .replace(duplicatedSlashesRegexp, "$1")

export const debounce = (func, delay) => {
  let timeout

  const debouncedFunction = (...args) => {
    clearTimeout(timeout)
    timeout = setTimeout(() => func(...args), delay)
  }

  // Add a cancel method to clear the timeout
  debouncedFunction.cancel = () => {
    clearTimeout(timeout)
  }

  return debouncedFunction
}
