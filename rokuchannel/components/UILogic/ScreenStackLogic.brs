sub InitScreenStack()
    m.screenStack = []
end sub

sub ShowScreen(node as Object)
    prev = m.screenStack.Peek()
    if prev <> invalid
        prev.visible = false
    end if
    m.top.AppendChild(node)
    node.visible = true
    node.SetFocus(true)
end sub

sub CloseScreen(node as Object)
    node.visible = false
    m.top.RemoveChild(node)
    prev = m.screenStack.Peek()
    if prev <> invalid
        prev.visible = true
        prev.SetFocus(true)
    end if
end sub