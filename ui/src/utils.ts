import { either, isEmpty, isNil } from "ramda";

export const isNilOrEmpty = either(isNil, isEmpty);

const duplicatedSlashesRegexp = new RegExp("(^\\/|[^:\\/]+\\/)\\/+", "g");

// Given some path segments returns a properly formatted path similarly to Nodejs path.join()
// Remove duplicated slashes
// Does not remove leading/trailing slashes and adds a slash between segments
export const pathJoin = (...pathParts) =>
  []
    .concat(...pathParts) // Flatten
    .filter((segment) => !!segment) // Remove empty parts
    .join("/")
    .replace(duplicatedSlashesRegexp, "$1");

/**
 * Checks if the given string is a valid Resource credential name.
 *
 * A valid Resource credential name is a string of up to 253 characters which
 * conforms to the following rules:
 *
 * 1. The first character must be a letter (a-z) or a number (0-9).
 * 2. The remaining characters must be letters, numbers, or hyphens.
 * 3. The string must end with a letter or number.
 *
 * @param {string} name The string to check.
 * @returns {boolean} Whether the string is a valid Resource credential name.
 */
export const isValidName = (name: string) =>
  (/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(name) &&
    name.length <= 253 &&
    name.length > 0) ||
  !name;

export const debounce = (func, delay) => {
  let timeout;

  const debouncedFunction = (...args) => {
    clearTimeout(timeout);
    timeout = setTimeout(() => func(...args), delay);
  };

  // Add a cancel method to clear the timeout
  debouncedFunction.cancel = () => {
    clearTimeout(timeout);
  };

  return debouncedFunction;
};
