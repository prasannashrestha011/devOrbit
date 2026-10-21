// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import YamlSource from "./YamlSource";

it("highlights YAML while keeping the textarea as the editable source", () => {
  const source = 'ports:\n  http: 8080 # listening\ncommand: "echo # okay"\n# note';
  const onChange = vi.fn();
  const { container } = render(<YamlSource value={source} onChange={onChange} />);
  const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
  const highlight = container.querySelector(".yaml-highlight") as HTMLElement;

  expect(textarea.value).toBe(source);
  expect(highlight.textContent).toBe(source);
  expect([...highlight.querySelectorAll(".yaml-key")].map((key) => key.textContent)).toEqual(["ports", "http", "command"]);
  expect(highlight.querySelector(".yaml-number")?.textContent?.trim()).toBe("8080");
  expect(highlight.querySelectorAll(".yaml-comment")).toHaveLength(2);
  fireEvent.change(textarea, { target: { value: "services: {}\n" } });
  expect(onChange).toHaveBeenCalledWith("services: {}\n");

  textarea.scrollTop = 40;
  textarea.scrollLeft = 12;
  fireEvent.scroll(textarea);
  expect(highlight.scrollTop).toBe(40);
  expect(highlight.scrollLeft).toBe(12);
});
