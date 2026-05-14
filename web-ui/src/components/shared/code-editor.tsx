import { useEffect, useRef } from "react";
import { EditorState, Compartment } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { syntaxHighlighting, defaultHighlightStyle, bracketMatching, foldGutter, indentOnInput } from "@codemirror/language";
import { json } from "@codemirror/lang-json";
import { yaml } from "@codemirror/lang-yaml";
import { oneDark } from "@codemirror/theme-one-dark";

interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  language?: "yaml" | "json";
  placeholder?: string;
  className?: string;
}

function getLangExtension(lang?: string) {
  switch (lang) {
    case "json":
      return json();
    case "yaml":
    default:
      return yaml();
  }
}

export default function CodeEditor({ value, onChange, language, placeholder, className }: CodeEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const langCompartment = useRef(new Compartment());
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    if (!containerRef.current) return;

    const state = EditorState.create({
      doc: value,
      extensions: [
        lineNumbers(),
        highlightActiveLine(),
        highlightActiveLineGutter(),
        foldGutter(),
        history(),
        bracketMatching(),
        indentOnInput(),
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        oneDark,
        langCompartment.current.of(getLangExtension(language)),
        keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            onChangeRef.current(update.state.doc.toString());
          }
        }),
        placeholder ? EditorView.contentAttributes.of({ "data-placeholder": placeholder }) : [],
        EditorView.theme({
          "&": { height: "100%", fontSize: "12px" },
          ".cm-scroller": { overflow: "auto" },
        }),
      ],
    });

    const view = new EditorView({ state, parent: containerRef.current });
    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== value) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: value },
      });
    }
  }, [value]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: langCompartment.current.reconfigure(getLangExtension(language)),
    });
  }, [language]);

  // Outer div takes layout space (flex-1, h-80, etc.)
  // Inner div uses absolute positioning to get exact pixel dimensions
  // cm-editor (position:relative !important, height:100%) fills the inner div
  return (
    <div className={`relative ${className ?? "h-80"}`}>
      <div
        ref={containerRef}
        className="absolute inset-0 overflow-hidden rounded-md border border-input"
      />
    </div>
  );
}
