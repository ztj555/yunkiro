import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import PhoneList from "../components/PhoneList";

describe("PhoneList", () => {
  it("shows empty message when no phones", () => {
    render(<PhoneList phones={[]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText("No phones connected")).toBeInTheDocument();
  });

  it("renders phone items", () => {
    const phones = [
      { device_id: "p1", device_name: "Phone A", online: true, status: "idle" as const },
      { device_id: "p2", device_name: "Phone B", online: false, status: "idle" as const },
    ];
    render(<PhoneList phones={phones} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText("Phone A")).toBeInTheDocument();
    expect(screen.getByText("Phone B")).toBeInTheDocument();
  });

  it("calls onSelect when phone clicked", () => {
    const onSelect = vi.fn();
    const phones = [
      { device_id: "p1", device_name: "Phone A", online: true, status: "idle" as const },
    ];
    render(<PhoneList phones={phones} selectedId={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByText("Phone A"));
    expect(onSelect).toHaveBeenCalledWith("p1");
  });

  it("highlights selected phone", () => {
    const phones = [
      { device_id: "p1", device_name: "Phone A", online: true, status: "idle" as const },
    ];
    const { container } = render(
      <PhoneList phones={phones} selectedId="p1" onSelect={vi.fn()} />
    );
    const item = container.querySelector(".phone-item.selected");
    expect(item).not.toBeNull();
  });
});
