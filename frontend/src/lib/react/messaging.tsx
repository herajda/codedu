"use client"

import { useState } from "react"
import { Send } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"

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
    <Card className="w-full max-w-md mx-auto h-[600px] flex flex-col">
      <CardHeader className="border-b p-4 flex flex-row items-center gap-3">
        <Avatar>
          <AvatarImage src="/placeholder.svg?height=40&width=40" alt="Contact" />
          <AvatarFallback>CN</AvatarFallback>
        </Avatar>
        <div className="flex flex-col">
          <h3 className="font-semibold">Alex Johnson</h3>
          <p className="text-xs text-muted-foreground">Online</p>
        </div>
      </CardHeader>
      <CardContent className="p-0 flex-1 overflow-hidden">
        <ScrollArea className="h-full p-4">
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
                      <Avatar className="h-8 w-8">
                        <AvatarImage src="/placeholder.svg?height=32&width=32" alt="Contact" />
                        <AvatarFallback>CN</AvatarFallback>
                      </Avatar>
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
        </ScrollArea>
      </CardContent>
      <CardFooter className="p-3 border-t">
        <form
          className="flex w-full items-center space-x-2"
          onSubmit={(e) => {
            e.preventDefault()
            handleSendMessage()
          }}
        >
          <Input
            placeholder="Type a message..."
            value={input}
            onChange={(e) => setInput(e.target.value)}
            className="flex-1"
          />
          <Button type="submit" size="icon" disabled={!input.trim()}>
            <Send className="h-4 w-4" />
            <span className="sr-only">Send</span>
          </Button>
        </form>
      </CardFooter>
    </Card>
  )
}
