import Editor from "@monaco-editor/react";
import { useEffect, useState } from "react";

type Props = {
  value: string;
  onChange: (value: string) => void;
};

function currentTheme() {
  return document.documentElement.dataset.theme === "dark" ? "vs-dark" : "vs-light";
}

export function YamlEditor({ value, onChange }: Props) {
  const [theme, setTheme] = useState(currentTheme);

  useEffect(() => {
    const observer = new MutationObserver(() => setTheme(currentTheme()));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    return () => observer.disconnect();
  }, []);

  return (
    <Editor
      height="420px"
      defaultLanguage="yaml"
      value={value}
      onChange={(next) => onChange(next ?? "")}
      theme={theme}
      options={{
        minimap: { enabled: false },
        fontSize: 14,
        roundedSelection: true,
        scrollBeyondLastLine: false,
      }}
    />
  );
}
