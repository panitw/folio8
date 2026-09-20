/// <summary>
/// What a render does with a font-chain entry naming a face the renderer was
/// never given — the managed spelling of Go's <c>folio8.FaceFallback</c>.
/// </summary>
/// <remarks>
/// The three outcomes are one rule, not three checks: an entry that resolves
/// renders and needs no font set at all; an entry that cannot resolve with
/// candidates available is refused or substituted according to this value; and
/// an entry that cannot resolve with no candidate at all is
/// <c>TEXT_FACE_ABSENT</c> either way.
/// <para>
/// The values are the C ABI's own, and they are kept in step with
/// <c>faceFallback</c> in <c>folio-go/cshared/cmd/folio8/main.go</c> and with
/// that ABI's README. <see cref="Strict"/> is zero, so the default of every
/// overload is the behaviour every call had before this argument existed.
/// </para>
/// </remarks>
public enum FaceFallback
{
    /// <summary>
    /// Refuse with <c>TEXT_FACE_ABSENT</c>. The default.
    /// </summary>
    Strict = 0,

    /// <summary>
    /// Paint the character in a face the renderer WAS given — the document's
    /// own embedded assets first, then the supplied <see cref="FontSet"/>,
    /// each in face-name order, the first face that covers the character
    /// winning — and report <c>TEXT_FACE_SUBSTITUTED</c> naming the element,
    /// the character, the face requested and the face painted. A renderer
    /// holding nothing that covers the character still refuses with
    /// <c>TEXT_FACE_ABSENT</c>.
    /// </summary>
    Substitute = 1,
}
