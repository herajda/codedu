"use client"

import { useState } from "react"
import { Send } from "lucide-react"
// The original component used UI primitives from a React
// library that is not part of this project.  In order to make
// the component usable inside the Svelte application without
// additional dependencies we replace those imports with plain
// HTML elements styled with Tailwind classes.

interface Message {
  id: string
  content: string
  sender: "user" | "other"
  timestamp: Date
  read: boolean
}

export function MessagingComponent() {
  const [input, setInput] = useState("")
  const [messages, setMessages] = useState<Message[]>([
    {
      id: "1",
      content: "Hey there! How's it going?",
      sender: "other",
      timestamp: new Date(Date.now() - 1000 * 60 * 60 * 24),
      read: true,
    },
    {
      id: "2",
      content: "I'm doing well, thanks for asking! How about you?",
      sender: "user",
      timestamp: new Date(Date.now() - 1000 * 60 * 60 * 23),
      read: true,
    },
    {
      id: "3",
      content: "Pretty good! Just working on some new projects.",
      sender: "other",
      timestamp: new Date(Date.now() - 1000 * 60 * 60 * 22),
      read: true,
    },
    {
      id: "4",
      content: "That sounds interesting! What kind of projects?",
      sender: "user",
      timestamp: new Date(Date.now() - 1000 * 60 * 60 * 21),
      read: true,
    },
    {
      id: "5",
      content: "Mostly web development stuff. I'm learning React and Next.js right now.",
      sender: "other",
      timestamp: new Date(Date.now() - 1000 * 60 * 60),
      read: false,
    },
  ])

  const handleSendMessage = () => {
    if (!input.trim()) return

    const newMessage: Message = {
      id: Date.now().toString(),
      content: input,
      sender: "user",
      timestamp: new Date(),
      read: false,
    }

    setMessages([...messages, newMessage])
    setInput("")
  }

  const formatTime = (date: Date) => {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
  }

  const formatDate = (date: Date) => {
    const today = new Date()
    const yesterday = new Date(today)
    yesterday.setDate(yesterday.getDate() - 1)

    if (date.toDateString() === today.toDateString()) {
      return "Today"
    } else if (date.toDateString() === yesterday.toDateString()) {
      return "Yesterday"
    } else {
      return date.toLocaleDateString()
    }
  }

  return (
    <div className="w-full max-w-md mx-auto h-[600px] flex flex-col border rounded-lg bg-background">
      <div className="border-b p-4 flex flex-row items-center gap-3">
        <div className="h-10 w-10 rounded-full bg-gray-200 overflow-hidden flex-shrink-0">
          <img src="/placeholder.svg?height=40&width=40" alt="Contact" />
        </div>
        <div className="flex flex-col">
          <h3 className="font-semibold">Alex Johnson</h3>
          <p className="text-xs text-muted-foreground">Online</p>
        </div>
      </div>
      <div className="p-0 flex-1 overflow-hidden">
        <div className="h-full p-4 overflow-y-auto space-y-4">
          {messages.map((message, index) => {
            const showDate = index === 0 || formatDate(message.timestamp) !== formatDate(messages[index - 1].timestamp)

            return (
              <div key={message.id} className="mb-4">
                {showDate && (
                  <div className="flex justify-center mb-4">
                    <span className="text-xs bg-muted px-2 py-1 rounded-md">{formatDate(message.timestamp)}</span>
                  </div>
                )}
                <div className={`flex ${message.sender === "user" ? "justify-end" : "justify-start"}`}>
                  <div className="flex gap-2 max-w-[80%]">
                  {message.sender === "other" && (
                      <div className="h-8 w-8 rounded-full bg-gray-200 overflow-hidden flex-shrink-0">
                        <img src="/placeholder.svg?height=32&width=32" alt="Contact" />
                      </div>
                    )}
                    <div>
                      <div
                        className={`rounded-lg p-3 ${
                          message.sender === "user" ? "bg-primary text-primary-foreground" : "bg-muted"
                        }`}
                      >
                        <p className="text-sm">{message.content}</p>
                      </div>
                      <div
                        className={`flex items-center mt-1 text-xs text-muted-foreground ${
                          message.sender === "user" ? "justify-end" : "justify-start"
                        }`}
                      >
                        <span>{formatTime(message.timestamp)}</span>
                        {message.sender === "user" && (
                          <span className="ml-1">{message.read ? "Read" : "Delivered"}</span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>
      <div className="p-3 border-t">
        <form
          className="flex w-full items-center space-x-2"
          onSubmit={(e) => {
            e.preventDefault()
            handleSendMessage()
          }}
        >
          <input
            placeholder="Type a message..."
            value={input}
            onChange={(e) => setInput(e.target.value)}
            className="flex-1 input input-bordered"
          />
          <button type="submit" className="btn btn-square" disabled={!input.trim()}>
            <Send className="h-4 w-4" />
            <span className="sr-only">Send</span>
          </button>
        </form>
      </div>
    </div>
  )
}
