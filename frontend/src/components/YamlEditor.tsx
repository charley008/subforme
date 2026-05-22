import Editor from "@monaco-editor/react";

type Props = {
  value: string;
  onChange: (value: string) => void;
};

export function YamlEditor({ value, onChange }: Props) {
  return (
    <Editor
      height="420px"
      defaultLanguage="yaml"
      value={value}
      onChange={(next) => onChange(next ?? "")}
      theme="vs-light"
      options={{
        minimap: { enabled: false },
        fontSize: 14,
        roundedSelection: true,
        scrollBeyondLastLine: false,
      }}
    />
  );
}
