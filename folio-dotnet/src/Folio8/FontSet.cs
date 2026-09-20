using System;
using System.Collections;
using System.Collections.Generic;

/// <summary>
/// The engine's explicit font input: a face name, as a template's fallback
/// chains name it, mapped to that face's raw OpenType/TrueType bytes.
/// </summary>
/// <remarks>
/// There is no default font set and no ambient lookup. The engine never
/// goes looking for fonts on the machine it runs on: a render either finds
/// the face it needs here, in the document's own embedded assets, or it
/// fails with a located error.
/// <para>
/// A font set <b>may be empty</b>. A document that carries every face it
/// names has nothing for a set to contribute, and no argument check refuses
/// such a call before the engine has attempted to resolve anything. What is
/// still a caller error is passing <c>null</c>.
/// </para>
/// <para>
/// Enumeration is in <b>insertion order</b>, not hash order, so the buffer
/// handed to the engine is a deterministic function of how the caller
/// built the set. The engine itself keys faces by name and does not see the
/// order; what it refuses is a duplicate name.
/// </para>
/// <para>
/// A face's bytes are <b>copied on the way in</b>, as
/// <see cref="Data"/>, <see cref="Params"/> and <see cref="Template"/> copy
/// theirs. Mutating the array you passed to <see cref="Add(string, byte[])"/>
/// afterwards cannot change what the engine renders, so a font set is as safe
/// to hold across calls as it looks.
/// </para>
/// </remarks>
public sealed class FontSet : IDictionary<string, byte[]>
{
    private readonly Dictionary<string, byte[]> _faces;
    private readonly List<string> _order;

    /// <summary>Creates an empty set.</summary>
    public FontSet()
    {
        _faces = new Dictionary<string, byte[]>(StringComparer.Ordinal);
        _order = new List<string>();
    }

    /// <summary>Creates a set holding the given faces, in the order they arrive.</summary>
    /// <param name="faces">The faces to copy in.</param>
    /// <exception cref="ArgumentNullException"><paramref name="faces"/> is null.</exception>
    public FontSet(IEnumerable<KeyValuePair<string, byte[]>> faces)
        : this()
    {
        if (faces == null)
        {
            throw new ArgumentNullException("faces");
        }
        foreach (KeyValuePair<string, byte[]> face in faces)
        {
            Add(face.Key, face.Value);
        }
    }

    /// <summary>The face bytes stored under <paramref name="name"/>.</summary>
    /// <param name="name">The face name.</param>
    /// <returns>The face's font program bytes.</returns>
    public byte[] this[string name]
    {
        get { return _faces[name]; }
        set
        {
            byte[] face = Validate(name, value);
            if (!_faces.ContainsKey(name))
            {
                _order.Add(name);
            }
            _faces[name] = face;
        }
    }

    /// <summary>The face names, in insertion order.</summary>
    public ICollection<string> Keys
    {
        get { return _order.ToArray(); }
    }

    /// <summary>The face bytes, in insertion order.</summary>
    public ICollection<byte[]> Values
    {
        get
        {
            byte[][] values = new byte[_order.Count][];
            for (int i = 0; i < _order.Count; i++)
            {
                values[i] = _faces[_order[i]];
            }
            return values;
        }
    }

    /// <summary>How many faces the set holds.</summary>
    public int Count
    {
        get { return _order.Count; }
    }

    /// <summary>Always false: a font set is built by its caller.</summary>
    public bool IsReadOnly
    {
        get { return false; }
    }

    /// <summary>Adds a face.</summary>
    /// <param name="name">The face name a template's fallback chain uses.</param>
    /// <param name="face">The face's raw font program bytes.</param>
    /// <exception cref="ArgumentNullException"><paramref name="name"/> or <paramref name="face"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="name"/> is empty, or already present.</exception>
    public void Add(string name, byte[] face)
    {
        byte[] copy = Validate(name, face);
        _faces.Add(name, copy);
        _order.Add(name);
    }

    /// <summary>Adds a face.</summary>
    /// <param name="face">The name and bytes.</param>
    public void Add(KeyValuePair<string, byte[]> face)
    {
        Add(face.Key, face.Value);
    }

    /// <summary>Empties the set.</summary>
    public void Clear()
    {
        _faces.Clear();
        _order.Clear();
    }

    /// <summary>Whether the set holds this entry.</summary>
    /// <param name="face">The entry to look for.</param>
    /// <returns><c>true</c> when the name is present with the same bytes.</returns>
    /// <remarks>
    /// Compared by CONTENT, not by reference: the set stores copies, so a
    /// reference comparison could never match what a caller passed in.
    /// </remarks>
    public bool Contains(KeyValuePair<string, byte[]> face)
    {
        byte[] stored;
        if (face.Key == null || !_faces.TryGetValue(face.Key, out stored))
        {
            return false;
        }
        if (face.Value == null || stored.Length != face.Value.Length)
        {
            return false;
        }
        for (int i = 0; i < stored.Length; i++)
        {
            if (stored[i] != face.Value[i])
            {
                return false;
            }
        }
        return true;
    }

    /// <summary>Whether a face of this name is present.</summary>
    /// <param name="name">The face name.</param>
    /// <returns><c>true</c> when it is present.</returns>
    public bool ContainsKey(string name)
    {
        return _faces.ContainsKey(name);
    }

    /// <summary>Copies the entries, in insertion order.</summary>
    /// <param name="array">The destination.</param>
    /// <param name="index">Where in the destination to start.</param>
    /// <exception cref="ArgumentNullException"><paramref name="array"/> is null.</exception>
    /// <exception cref="ArgumentOutOfRangeException"><paramref name="index"/> is negative.</exception>
    /// <exception cref="ArgumentException">The set does not fit from <paramref name="index"/>.</exception>
    public void CopyTo(KeyValuePair<string, byte[]>[] array, int index)
    {
        // The ICollection contract's exceptions, thrown BEFORE anything is
        // written: a partial copy followed by IndexOutOfRangeException leaves
        // the caller's array in a state neither of us can describe.
        if (array == null)
        {
            throw new ArgumentNullException("array");
        }
        if (index < 0)
        {
            throw new ArgumentOutOfRangeException("index", index, "folio8: the destination index may not be negative");
        }
        if (array.Length - index < Count)
        {
            throw new ArgumentException("folio8: the destination array has room for " + (array.Length - index) + " of " + Count + " faces", "array");
        }
        foreach (KeyValuePair<string, byte[]> face in this)
        {
            array[index++] = face;
        }
    }

    /// <summary>Enumerates the faces in insertion order.</summary>
    /// <returns>The enumerator.</returns>
    public IEnumerator<KeyValuePair<string, byte[]>> GetEnumerator()
    {
        for (int i = 0; i < _order.Count; i++)
        {
            yield return new KeyValuePair<string, byte[]>(_order[i], _faces[_order[i]]);
        }
    }

    IEnumerator IEnumerable.GetEnumerator()
    {
        return GetEnumerator();
    }

    /// <summary>Removes a face by name.</summary>
    /// <param name="name">The face name.</param>
    /// <returns><c>true</c> when it was present.</returns>
    public bool Remove(string name)
    {
        if (!_faces.Remove(name))
        {
            return false;
        }
        _order.Remove(name);
        return true;
    }

    /// <summary>Removes this exact entry.</summary>
    /// <param name="face">The entry to remove.</param>
    /// <returns><c>true</c> when it was present.</returns>
    public bool Remove(KeyValuePair<string, byte[]> face)
    {
        return Contains(face) && Remove(face.Key);
    }

    /// <summary>Looks a face up.</summary>
    /// <param name="name">The face name.</param>
    /// <param name="face">The bytes, when present.</param>
    /// <returns><c>true</c> when it was present.</returns>
    public bool TryGetValue(string name, out byte[] face)
    {
        return _faces.TryGetValue(name, out face);
    }

    /// <summary>Checks an entry and returns the copy the set will store.</summary>
    private static byte[] Validate(string name, byte[] face)
    {
        if (name == null)
        {
            throw new ArgumentNullException("name");
        }
        if (name.Length == 0)
        {
            throw new ArgumentException("folio8: a face name may not be empty", "name");
        }
        if (face == null)
        {
            throw new ArgumentNullException("face");
        }
        byte[] copy = new byte[face.Length];
        Buffer.BlockCopy(face, 0, copy, 0, face.Length);
        return copy;
    }
}
