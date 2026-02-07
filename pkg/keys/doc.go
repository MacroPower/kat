// Package keys provides utilities for key(board) handling and configuration.
//
// [KeyBind] is the canonical key binding type used throughout kat. It is
// JSON-serializable and user-customizable. When interfacing with bubbles
// components that require [key.Binding], use [KeyBind.BubbleKey] to convert
// at the boundary. To convert in the other direction, use [FromBubbleKey].
package keys
