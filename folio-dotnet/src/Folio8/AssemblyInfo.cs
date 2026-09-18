using System.Runtime.CompilerServices;

// The test project reads two internals and nothing else: the outstanding
// native allocation count (which is how free discipline is PROVED over the
// corpus loop rather than asserted) and the native entry points, so a
// double free can be exercised at the ABI rather than described in a comment.
[assembly: InternalsVisibleTo("Folio8.Tests")]
