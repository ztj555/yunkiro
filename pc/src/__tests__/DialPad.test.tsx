import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import DialPad from "../components/DialPad";

describe("DialPad", () => {
  const defaultProps = {
    selectedPhone: "phone-1",
    onDial: vi.fn(),
    onHangup: vi.fn(),
    detectedNumber: null,
    onClearDetected: vi.fn(),
  };

  it("renders number input", () => {
    render(<DialPad {...defaultProps} />);
    expect(screen.getByPlaceholderText("Enter phone number")).toBeInTheDocument();
  });

  it("dial button is disabled when number is invalid", () => {
    render(<DialPad {...defaultProps} />);
    const dialBtn = screen.getByText("Dial");
    expect(dialBtn).toBeDisabled();
  });

  it("dial button is enabled for valid 11-digit number starting with 1", () => {
    render(<DialPad {...defaultProps} />);
    const input = screen.getByPlaceholderText("Enter phone number");
    fireEvent.change(input, { target: { value: "13800138000" } });
    const dialBtn = screen.getByText("Dial");
    expect(dialBtn).not.toBeDisabled();
  });

  it("dial button is disabled when no phone selected", () => {
    render(<DialPad {...defaultProps} selectedPhone={null} />);
    const input = screen.getByPlaceholderText("Enter phone number");
    fireEvent.change(input, { target: { value: "13800138000" } });
    const dialBtn = screen.getByText("Dial");
    expect(dialBtn).toBeDisabled();
  });

  it("calls onDial with number and simSlot when dial button clicked", () => {
    const onDial = vi.fn();
    render(<DialPad {...defaultProps} onDial={onDial} />);
    const input = screen.getByPlaceholderText("Enter phone number");
    fireEvent.change(input, { target: { value: "13800138000" } });
    fireEvent.click(screen.getByText("Dial"));
    expect(onDial).toHaveBeenCalledWith("13800138000", 0);
  });

  it("fills detected number from clipboard", () => {
    const onClearDetected = vi.fn();
    render(
      <DialPad
        {...defaultProps}
        detectedNumber="13912345678"
        onClearDetected={onClearDetected}
      />
    );
    const input = screen.getByPlaceholderText("Enter phone number") as HTMLInputElement;
    expect(input.value).toBe("13912345678");
    expect(onClearDetected).toHaveBeenCalled();
  });

  it("keypad buttons append digits to input", () => {
    render(<DialPad {...defaultProps} />);
    fireEvent.click(screen.getByText("1"));
    fireEvent.click(screen.getByText("3"));
    fireEvent.click(screen.getByText("8"));
    const input = screen.getByPlaceholderText("Enter phone number") as HTMLInputElement;
    expect(input.value).toBe("138");
  });
});
