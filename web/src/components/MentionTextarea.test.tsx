import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import type { Member } from "../api";
import { MentionTextarea } from "./MentionTextarea";

const members: Member[] = [
  { user_id: "u-1", email: "ali@kyber.dev", name: "Ali Valiyev", role: "admin" },
  { user_id: "u-2", email: "bobur@kyber.dev", name: "Bobur Karimov", role: "member" },
  { user_id: "u-3", email: "alisher@kyber.dev", name: "Alisher Usmonov", role: "viewer" },
];

function Harness() {
  const [v, setV] = useState("");
  return (
    <>
      <MentionTextarea label="Add a comment" value={v} onChange={setV} members={members} />
      <output data-testid="value">{v}</output>
    </>
  );
}

describe("@mention autocomplete (KYB-S31)", () => {
  it("AC1: typing @ lists matching members by name or email", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const box = screen.getByRole("combobox", { name: "Add a comment" });
    expect(box).toHaveAttribute("aria-expanded", "false");
    await user.type(box, "hi @ali");
    expect(box).toHaveAttribute("aria-expanded", "true");
    const opts = screen.getAllByRole("option");
    expect(opts.map((o) => o.textContent)).toEqual(["Ali Valiyevali@kyber.dev", "Alisher Usmonovalisher@kyber.dev"]);
    expect(opts[0]).toHaveAttribute("aria-selected", "true");
  });

  it("AC2: arrows move, Enter inserts @email and closes", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const box = screen.getByRole("combobox", { name: "Add a comment" });
    await user.type(box, "ping @al");
    await user.keyboard("{ArrowDown}");
    expect(screen.getAllByRole("option")[1]).toHaveAttribute("aria-selected", "true");
    expect(box.getAttribute("aria-activedescendant")).toBe(screen.getAllByRole("option")[1]?.id);
    await user.keyboard("{Enter}");
    expect(screen.getByTestId("value")).toHaveTextContent("ping @alisher@kyber.dev");
    expect(box).toHaveAttribute("aria-expanded", "false");
    await user.type(box, "thanks");
    expect(screen.getByTestId("value").textContent).toBe("ping @alisher@kyber.dev thanks");
  });

  it("AC3: click picks, Escape dismisses, no match shows nothing", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const box = screen.getByRole("combobox", { name: "Add a comment" });
    await user.type(box, "@bob");
    await user.click(screen.getByRole("option", { name: /Bobur/ }));
    expect(screen.getByTestId("value").textContent).toBe("@bobur@kyber.dev ");
    await user.type(box, "@a");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    await user.type(box, " @zzz");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("AC4: an email address in prose does not trigger suggestions", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.type(screen.getByRole("combobox", { name: "Add a comment" }), "mail me at me@al");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });
});
